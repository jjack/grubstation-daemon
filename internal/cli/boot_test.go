//go:build linux

package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jjack/grubstation/internal/config"
	"github.com/jjack/grubstation/internal/daemon"
	"github.com/jjack/grubstation/internal/grub"
	"github.com/jjack/grubstation/internal/servicemanager"
)

func TestBootListCmd(t *testing.T) {
	tempGrub := t.TempDir() + "/grub.cfg"
	_ = os.WriteFile(tempGrub, []byte("menuentry 'Ubuntu' {}\nmenuentry 'Windows' {}"), 0o644)

	deps := &CommandDeps{
		Grub: &grub.Grub{ConfigPath: tempGrub},
	}

	cmd := NewBootListCmd(deps)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Available Boot Options:") {
		t.Errorf("expected header, got %q", out.String())
	}
	if !strings.Contains(out.String(), "- Ubuntu") {
		t.Errorf("expected Ubuntu, got %q", out.String())
	}
	if !strings.Contains(out.String(), "- Windows") {
		t.Errorf("expected Windows, got %q", out.String())
	}
}

func TestBootListCmd_Empty(t *testing.T) {
	tempGrub := t.TempDir() + "/grub.cfg"
	_ = os.WriteFile(tempGrub, []byte(""), 0o644)

	deps := &CommandDeps{
		Grub: &grub.Grub{ConfigPath: tempGrub},
	}

	cmd := NewBootListCmd(deps)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "(None found)") {
		t.Errorf("expected (None found), got %q", out.String())
	}
}

func TestBootPushCmd_Direct(t *testing.T) {
	// Mock HA server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer ts.Close()

	tempGrub := t.TempDir() + "/grub.cfg"
	_ = os.WriteFile(tempGrub, []byte("menuentry 'Ubuntu' {}"), 0o644)

	initReg := servicemanager.NewRegistry()
	initReg.Register("mock-init", func() servicemanager.Manager { return &mockServiceManager{name: "mock-init"} })

	deps := &CommandDeps{
		Config:   &config.Config{HomeAssistant: config.HomeAssistantConfig{URL: ts.URL, WebhookID: "fake"}},
		Grub:     &grub.Grub{ConfigPath: tempGrub},
		Registry: initReg,
	}

	// Ensure no socket exists to force direct push
	oldSocketPath := daemon.SocketPath
	daemon.SocketPath = filepath.Join(t.TempDir(), "non-existent.sock")
	defer func() { daemon.SocketPath = oldSocketPath }()

	cmd := NewBootPushCmd(deps)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Successfully pushed boot options to Home Assistant") {
		t.Errorf("expected success message, got %q", out.String())
	}
}

func TestBootPushCmd_Socket(t *testing.T) {
	oldSocketPath := daemon.SocketPath
	daemon.SocketPath = filepath.Join(t.TempDir(), "test.sock")
	defer func() { daemon.SocketPath = oldSocketPath }()

	// Start a dummy unix socket server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := daemon.New(daemon.Config{}, daemon.Metadata{}, nil, func(ctx context.Context) error {
		return nil
	})
	go func() { _ = d.Run(ctx) }()

	// Wait for socket
	found := false
	for i := 0; i < 20; i++ {
		if _, err := os.Stat(daemon.SocketPath); err == nil {
			found = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !found {
		t.Fatal("socket was never created")
	}

	deps := &CommandDeps{}
	cmd := NewBootPushCmd(deps)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Successfully pushed boot options to Home Assistant (via running daemon)") {
		t.Errorf("expected socket success message, got %q", out.String())
	}
}
