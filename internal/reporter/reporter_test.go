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

	err := r.PushBootOptions(context.Background(), "token")
	if err != ErrMissingHAConfig {
		t.Errorf("expected ErrMissingHAConfig, got %v", err)
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
	var receivedPayload ha.PushPayload
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
			Name:       "test-host",
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
	err = r.PushBootOptions(context.Background(), "tofu-token")
	if err != nil {
		t.Fatalf("PushBootOptions failed: %v", err)
	}

	// 5. Verify
	if receivedPayload.Name != "test-host" {
		t.Errorf("expected host test-host, got %s", receivedPayload.Name)
	}
	if receivedPayload.APIToken != "tofu-token" {
		t.Errorf("expected token tofu-token, got %s", receivedPayload.APIToken)
	}
	if len(receivedPayload.BootOptions) != 2 {
		t.Errorf("expected 2 boot options, got %d", len(receivedPayload.BootOptions))
	}
	if receivedPayload.BootOptions[0] != "Linux" || receivedPayload.BootOptions[1] != "Windows" {
		t.Errorf("unexpected boot options: %v", receivedPayload.BootOptions)
	}
	if receivedPayload.ServiceManager != "test-manager" {
		t.Errorf("expected service manager test-manager, got %s", receivedPayload.ServiceManager)
	}
}

func TestReporter_PushBootOptions_NoGrubReporting(t *testing.T) {
	var receivedPayload ha.PushPayload
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
	err := r.PushBootOptions(context.Background(), "token")
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

	err := r.PushBootOptions(context.Background(), "token")
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

	err := r.PushBootOptions(context.Background(), "token")
	if err == nil {
		t.Fatal("expected error when HA push fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to push boot options to HA webhook") {
		t.Errorf("expected error message to contain 'failed to push boot options to HA webhook', got: %v", err)
	}
}
