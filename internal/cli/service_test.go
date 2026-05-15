package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/jjack/grubstation/internal/config"
	"github.com/jjack/grubstation/internal/grub"
	"github.com/jjack/grubstation/internal/servicemanager"
)

type mockServiceManager struct {
	name         string
	activeCalls  int
	active       bool
	installed    bool
	installErr   error
	uninstallErr error
	startErr     error
	stopErr      error
}

func (m *mockServiceManager) Name() string { return m.name }
func (m *mockServiceManager) IsActive(ctx context.Context) bool {
	res := m.active
	if m.activeCalls == 0 {
		res = true // Force true for Detect
	}
	m.activeCalls++
	return res
}
func (m *mockServiceManager) IsInstalled(ctx context.Context) (bool, error) {
	return m.installed, nil
}
func (m *mockServiceManager) CheckPermissions(ctx context.Context) error { return nil }
func (m *mockServiceManager) Install(ctx context.Context, configPath string) error {
	return m.installErr
}
func (m *mockServiceManager) Uninstall(ctx context.Context) error { return m.uninstallErr }
func (m *mockServiceManager) Start(ctx context.Context) error     { return m.startErr }
func (m *mockServiceManager) Stop(ctx context.Context) error      { return m.stopErr }

func TestServiceInstallCmd(t *testing.T) {
	initReg := servicemanager.NewRegistry()
	mock := &mockServiceManager{name: "mock-svc", active: true}
	initReg.Register("mock-svc", func() servicemanager.Manager { return mock })

	deps := &CommandDeps{
		Config:   &config.Config{},
		Registry: initReg,
	}

	cmd := NewServiceInstallCmd(deps)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.Flags().String("config", "config.yaml", "")

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Installing service: mock-svc") {
		t.Errorf("expected installing message, got %q", out.String())
	}
	if !strings.Contains(out.String(), "Installation completed successfully.") {
		t.Errorf("expected success message, got %q", out.String())
	}
}

func TestServiceRemoveCmd(t *testing.T) {
	initReg := servicemanager.NewRegistry()
	mock := &mockServiceManager{name: "mock-svc", active: true}
	initReg.Register("mock-svc", func() servicemanager.Manager { return mock })

	grub.HassGrubStationPath = t.TempDir() + "/99_grubstation"
	deps := &CommandDeps{
		Config:   &config.Config{Daemon: config.DaemonConfig{ReportBootOptions: true}},
		Grub:     &grub.Grub{ConfigPath: t.TempDir() + "/grub.cfg"},
		Registry: initReg,
	}

	cmd := NewServiceRemoveCmd(deps)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Removing service: mock-svc") {
		t.Errorf("expected removing message, got %q", out.String())
	}
	if !strings.Contains(out.String(), "Removal completed successfully.") {
		t.Errorf("expected success message, got %q", out.String())
	}
}

func TestServiceStartCmd(t *testing.T) {
	initReg := servicemanager.NewRegistry()
	mock := &mockServiceManager{name: "mock-svc", active: true}
	initReg.Register("mock-svc", func() servicemanager.Manager { return mock })

	grub.HassGrubStationPath = t.TempDir() + "/99_grubstation"
	deps := &CommandDeps{Registry: initReg}
	cmd := NewServiceStartCmd(deps)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceStopCmd(t *testing.T) {
	initReg := servicemanager.NewRegistry()
	mock := &mockServiceManager{name: "mock-svc", active: true}
	initReg.Register("mock-svc", func() servicemanager.Manager { return mock })

	grub.HassGrubStationPath = t.TempDir() + "/99_grubstation"
	deps := &CommandDeps{Registry: initReg}
	cmd := NewServiceStopCmd(deps)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceStatusCmd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthcheck" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ALIVE"))
		}
	}))
	defer ts.Close()

	// Extract port from ts.URL
	var port int
	fmt.Sscanf(ts.URL, "http://127.0.0.1:%d", &port)

	initReg := servicemanager.NewRegistry()
	mock := &mockServiceManager{name: "mock-svc", active: true}
	initReg.Register("mock-svc", func() servicemanager.Manager { return mock })

	grub.HassGrubStationPath = t.TempDir() + "/99_grubstation"
	deps := &CommandDeps{
		Config:   &config.Config{Daemon: config.DaemonConfig{Port: port}},
		Registry: initReg,
	}

	cmd := NewServiceStatusCmd(deps)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Service mock-svc is active") {
		t.Errorf("expected active message, got %q", out.String())
	}
	if !strings.Contains(out.String(), "Daemon health: ALIVE") {
		t.Errorf("expected health message, got %q", out.String())
	}
}

func TestServiceStatusCmd_Inactive(t *testing.T) {
	initReg := servicemanager.NewRegistry()
	mock := &mockServiceManager{name: "mock-svc", active: false}
	initReg.Register("mock-svc", func() servicemanager.Manager { return mock })

	grub.HassGrubStationPath = t.TempDir() + "/99_grubstation"
	deps := &CommandDeps{
		Config:   &config.Config{Daemon: config.DaemonConfig{Port: 0}}, // Port 0 will fail
		Registry: initReg,
	}

	cmd := NewServiceStatusCmd(deps)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Service mock-svc is inactive") {
		t.Errorf("expected inactive message, got %q", out.String())
	}
	if !strings.Contains(out.String(), "Daemon health check failed") {
		t.Errorf("expected failed health check message, got %q", out.String())
	}
}

func TestServiceStatusCmd_NonOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	var port int
	fmt.Sscanf(ts.URL, "http://127.0.0.1:%d", &port)

	initReg := servicemanager.NewRegistry()
	mock := &mockServiceManager{name: "mock-svc", active: true}
	initReg.Register("mock-svc", func() servicemanager.Manager { return mock })

	grub.HassGrubStationPath = t.TempDir() + "/99_grubstation"
	deps := &CommandDeps{
		Config:   &config.Config{Daemon: config.DaemonConfig{Port: port}},
		Registry: initReg,
	}

	cmd := NewServiceStatusCmd(deps)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Daemon health check returned non-OK status: 404") {
		t.Errorf("expected 404 health check message, got %q", out.String())
	}
}

func TestServiceCmd(t *testing.T) {
	grub.HassGrubStationPath = t.TempDir() + "/99_grubstation"
	deps := &CommandDeps{}
	cmd := NewServiceCmd(deps)
	if cmd.Use != "service" {
		t.Errorf("expected Use 'service', got %q", cmd.Use)
	}
}

func TestServiceRemoveCmd_Error(t *testing.T) {
	initReg := servicemanager.NewRegistry()
	mock := &mockServiceManager{name: "mock-svc", active: true, uninstallErr: errors.New("uninstall failed")}
	initReg.Register("mock-svc", func() servicemanager.Manager { return mock })

	grub.HassGrubStationPath = t.TempDir() + "/99_grubstation"
	deps := &CommandDeps{
		Config:   &config.Config{},
		Registry: initReg,
	}

	cmd := NewServiceRemoveCmd(deps)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "uninstall failed") {
		t.Fatalf("expected uninstall error, got %v", err)
	}
}

func TestServiceRemoveCmd_GrubError(t *testing.T) {
	oldExecLookPath := grub.ExecLookPath
	oldExecCommand := grub.ExecCommand
	defer func() {
		grub.ExecLookPath = oldExecLookPath
		grub.ExecCommand = oldExecCommand
	}()

	grub.ExecLookPath = func(file string) (string, error) {
		if file == "update-grub" {
			return "/bin/false", nil
		}
		return "", errors.New("not found")
	}
	grub.ExecCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	initReg := servicemanager.NewRegistry()
	mock := &mockServiceManager{name: "mock-svc", active: true}
	initReg.Register("mock-svc", func() servicemanager.Manager { return mock })

	grub.HassGrubStationPath = t.TempDir() + "/99_grubstation"
	deps := &CommandDeps{
		Config:   &config.Config{Daemon: config.DaemonConfig{ReportBootOptions: true}},
		Grub:     &grub.Grub{ConfigPath: "/invalid/path"},
		Registry: initReg,
	}

	cmd := NewServiceRemoveCmd(deps)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "failed to uninstall grub") {
		t.Fatalf("expected grub uninstall error, got %v", err)
	}
}

func TestServiceStartCmd_Error(t *testing.T) {
	initReg := servicemanager.NewRegistry()
	// No services registered, Detect will fail
	deps := &CommandDeps{Registry: initReg}
	cmd := NewServiceStartCmd(deps)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestServiceStopCmd_Error(t *testing.T) {
	initReg := servicemanager.NewRegistry()
	deps := &CommandDeps{Registry: initReg}
	cmd := NewServiceStopCmd(deps)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestServiceStatusCmd_Error(t *testing.T) {
	initReg := servicemanager.NewRegistry()
	deps := &CommandDeps{Registry: initReg}
	cmd := NewServiceStatusCmd(deps)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error, got nil")
	}
}
