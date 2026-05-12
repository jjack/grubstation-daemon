package reporter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjack/grubstation-daemon/internal/config"
	"github.com/jjack/grubstation-daemon/internal/grub"
	ha "github.com/jjack/grubstation-daemon/internal/homeassistant"
)

func TestReporter_PushBootOptions_MissingConfig(t *testing.T) {
	cfg := &config.Config{}
	r := New(cfg, nil, "test-manager")

	err := r.PushBootOptions(context.Background())
	if err != ErrMissingHAConfig {
		t.Errorf("expected ErrMissingHAConfig, got %v", err)
	}
}

func TestReporter_RegisterDaemon_Success(t *testing.T) {
	// 1. Setup mock HA server
	var receivedPayload ha.RegistrationPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/webhook/webhook123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&receivedPayload); err != nil {
			t.Errorf("failed to decode payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	// 2. Configure reporter
	cfg := &config.Config{
		Host: config.HostConfig{
			Address:    "192.168.1.10",
			MACAddress: "AA:BB:CC:DD:EE:FF",
		},
		WakeOnLan: config.WakeOnLanConfig{
			Address: "192.168.1.255",
			Port:    9,
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       server.URL,
			WebhookID: "webhook123",
		},
		Daemon: config.DaemonConfig{
			Port: 8081,
		},
	}

	r := New(cfg, nil, "test-manager")

	// 3. Execute
	err := r.RegisterDaemon(context.Background(), "tofu-token")
	if err != nil {
		t.Fatalf("RegisterDaemon failed: %v", err)
	}

	// 4. Verify
	if receivedPayload.Action != ha.ActionRegisterAction {
		t.Errorf("expected action register_daemon, got %s", receivedPayload.Action)
	}
	if receivedPayload.DaemonToken != "tofu-token" {
		t.Errorf("expected token tofu-token, got %s", receivedPayload.DaemonToken)
	}
	if receivedPayload.DaemonPort != 8081 {
		t.Errorf("expected port 8081, got %d", receivedPayload.DaemonPort)
	}
	if receivedPayload.ServiceManager != "test-manager" {
		t.Errorf("expected service manager test-manager, got %s", receivedPayload.ServiceManager)
	}
}

func TestReporter_PushBootOptions_Success(t *testing.T) {
	// 1. Setup mock GRUB config
	tmpDir, err := os.MkdirTemp("", "reporter-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	grubCfgPath := filepath.Join(tmpDir, "grub.cfg")
	grubContent := `
menuentry 'Linux' {
    set root='hd0,msdos1'
}
menuentry 'Windows' {
    set root='hd0,msdos2'
}
`
	if err := os.WriteFile(grubCfgPath, []byte(grubContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. Setup mock HA server
	var receivedPayload ha.UpdatePayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/webhook/webhook123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&receivedPayload); err != nil {
			t.Errorf("failed to decode payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	// 3. Configure reporter
	cfg := &config.Config{
		Host: config.HostConfig{
			Address:    "192.168.1.10",
			MACAddress: "AA:BB:CC:DD:EE:FF",
		},
		WakeOnLan: config.WakeOnLanConfig{
			Address: "192.168.1.255",
			Port:    9,
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       server.URL,
			WebhookID: "webhook123",
		},
		Daemon: config.DaemonConfig{
			ReportBootOptions: true,
		},
	}

	g := &grub.Grub{ConfigPath: grubCfgPath}
	r := New(cfg, g, "test-manager")

	// 4. Execute
	err = r.PushBootOptions(context.Background())
	if err != nil {
		t.Fatalf("PushBootOptions failed: %v", err)
	}

	// 5. Verify
	if receivedPayload.Action != ha.ActionUpdateAction {
		t.Errorf("expected action update_boot_options, got %s", receivedPayload.Action)
	}
	if receivedPayload.ServiceManager != "test-manager" {
		t.Errorf("expected service manager test-manager, got %s", receivedPayload.ServiceManager)
	}
	if receivedPayload.WolBroadcastAddress != "192.168.1.255" {
		t.Errorf("expected broadcast address 192.168.1.255, got %s", receivedPayload.WolBroadcastAddress)
	}
	if receivedPayload.WolBroadcastPort != 9 {
		t.Errorf("expected broadcast port 9, got %d", receivedPayload.WolBroadcastPort)
	}
	if len(receivedPayload.BootOptions) != 2 {
		t.Errorf("expected 2 boot options, got %d", len(receivedPayload.BootOptions))
	}
	if receivedPayload.BootOptions[0] != "Linux" || receivedPayload.BootOptions[1] != "Windows" {
		t.Errorf("unexpected boot options: %v", receivedPayload.BootOptions)
	}
}

func TestReporter_PushBootOptions_NoGrubReporting(t *testing.T) {
	var receivedPayload ha.UpdatePayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&receivedPayload); err != nil {
			t.Errorf("failed to decode: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	cfg := &config.Config{
		HomeAssistant: config.HomeAssistantConfig{URL: server.URL, WebhookID: "id"},
		Daemon:        config.DaemonConfig{ReportBootOptions: false},
	}
	r := New(cfg, nil, "manager")
	err := r.PushBootOptions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(receivedPayload.BootOptions) != 0 {
		t.Errorf("expected 0 boot options, got %d", len(receivedPayload.BootOptions))
	}
}

func TestReporter_PushBootOptions_GrubError(t *testing.T) {
	cfg := &config.Config{
		Daemon: config.DaemonConfig{
			ReportBootOptions: true,
		},
	}
	// Use an invalid path to trigger GetBootOptions error
	g := &grub.Grub{ConfigPath: "/non/existent/path/grub.cfg"}
	r := New(cfg, g, "test-manager")

	err := r.PushBootOptions(context.Background())
	if err == nil {
		t.Fatal("expected error for missing grub config, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get boot options") {
		t.Errorf("expected error message to contain 'failed to get boot options', got: %v", err)
	}
}

func TestReporter_PushBootOptions_PushError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.Config{
		HomeAssistant: config.HomeAssistantConfig{
			URL:       server.URL,
			WebhookID: "webhook123",
		},
		Daemon: config.DaemonConfig{
			ReportBootOptions: false,
		},
	}
	r := New(cfg, nil, "test-manager")

	err := r.PushBootOptions(context.Background())
	if err == nil {
		t.Fatal("expected error when HA push fails, got nil")
	}
}
