package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jjack/remote-boot-agent/internal/config"
	"github.com/jjack/remote-boot-agent/internal/grub"
	ha "github.com/jjack/remote-boot-agent/internal/homeassistant"
)

// createPushTempGrubConfig creates a temporary grub config file and returns its path and a cleanup function.
func createPushTempGrubConfig(t *testing.T) string {
	tempGrub, err := os.CreateTemp("", "grub.cfg")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = tempGrub.Write([]byte("menuentry 'Test OS' {}\n"))
	_ = tempGrub.Close()
	t.Cleanup(func() { _ = os.Remove(tempGrub.Name()) })
	return tempGrub.Name()
}

func TestPushBootOptionsCommand(t *testing.T) {
	var payload ha.PushPayload

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to parse json: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	tempGrubPath := createPushTempGrubConfig(t)
	cfg := &config.Config{
		Host: config.HostConfig{
			MACAddress:       "aa:bb:cc:dd:ee:ff",
			BroadcastAddress: "192.168.1.255",
			BroadcastPort:    7, // use a custom port to ensure it gets passed
			Name:             "test-name",
			Address:          "10.0.0.1",
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       ts.URL,
			WebhookID: "test-webhook",
		},
	}

	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{ConfigPath: tempGrubPath}}
	cmd := NewPushCmd(deps)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if payload.MACAddress != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("expected MAC address aa:bb:cc:dd:ee:ff, got %s", payload.MACAddress)
	}
	if payload.BroadcastAddress != "192.168.1.255" {
		t.Errorf("expected broadcast address 192.168.1.255, got %s", payload.BroadcastAddress)
	}
	if payload.BroadcastPort != 7 {
		t.Errorf("expected custom WOL port 7, got %d", payload.BroadcastPort)
	}
	if payload.Name != "test-name" {
		t.Errorf("expected name test-name, got %s", payload.Name)
	}
	if payload.Address != "10.0.0.1" {
		t.Errorf("expected address 10.0.0.1, got %s", payload.Address)
	}
	if len(payload.BootOptions) != 1 || payload.BootOptions[0] != "Test OS" {
		t.Errorf("expected [Test OS], got %v", payload.BootOptions)
	}
}

func TestPushBootOptionsCommand_DefaultWOL(t *testing.T) {
	var payload ha.PushPayload

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to parse json: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	tempGrubPath := createPushTempGrubConfig(t)
	cfg := &config.Config{
		Host: config.HostConfig{
			MACAddress:       "aa:bb:cc:dd:ee:ff",
			BroadcastAddress: config.DefaultBroadcastAddress,
			BroadcastPort:    config.DefaultBroadcastPort,
			Name:             "test-name",
			Address:          "10.0.0.1",
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       ts.URL,
			WebhookID: "test-webhook",
		},
	}

	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{ConfigPath: tempGrubPath}}
	cmd := NewPushCmd(deps)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Ensure the defaults are mapped to empty/zero so they get stripped via omitempty in the JSON
	if payload.BroadcastAddress != "" {
		t.Errorf("expected broadcast address to be omitted (empty string), got %s", payload.BroadcastAddress)
	}
	if payload.BroadcastPort != 0 {
		t.Errorf("expected WOL port to be omitted (0), got %d", payload.BroadcastPort)
	}
}

func TestPushBootOptionsCommand_ZeroWOL(t *testing.T) {
	var payload ha.PushPayload

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to parse json: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	tempGrubPath := createPushTempGrubConfig(t)
	cfg := &config.Config{
		Host: config.HostConfig{
			MACAddress:       "aa:bb:cc:dd:ee:ff",
			BroadcastAddress: "", // Zero value instead of default
			BroadcastPort:    0,  // Zero value instead of default
			Name:             "test-name",
			Address:          "10.0.0.1",
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       ts.URL,
			WebhookID: "test-webhook",
		},
	}

	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{ConfigPath: tempGrubPath}}
	cmd := NewPushCmd(deps)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Ensure the empty/zero values are handled correctly so they get stripped via omitempty in the JSON
	if payload.BroadcastAddress != "" {
		t.Errorf("expected broadcast address to be omitted (empty string), got %s", payload.BroadcastAddress)
	}
	if payload.BroadcastPort != 0 {
		t.Errorf("expected WOL port to be omitted (0), got %d", payload.BroadcastPort)
	}
}

func TestPushBootOptionsCommand_GrubError(t *testing.T) {
	cfg := &config.Config{}

	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{ConfigPath: "/invalid/path/grub.cfg"}}
	cmd := NewPushCmd(deps)
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error from GetBootOptions, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get boot options") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPushBootOptionsCommand_HAClientError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	tempGrubPath := createPushTempGrubConfig(t)
	cfg := &config.Config{
		HomeAssistant: config.HomeAssistantConfig{URL: ts.URL, WebhookID: "test"},
	}
	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{ConfigPath: tempGrubPath}}
	cmd := NewPushCmd(deps)
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error from HA Push, got nil")
	}
}

func TestPushBootOptionsCommand_MissingHAConfig(t *testing.T) {
	tempGrubPath := createPushTempGrubConfig(t)
	cfg := &config.Config{
		HomeAssistant: config.HomeAssistantConfig{
			URL: "",
		},
	}

	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{ConfigPath: tempGrubPath}}
	cmd := NewPushCmd(deps)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error due to missing HA config, got nil")
	}
	if !strings.Contains(err.Error(), "homeassistant url and webhook_id must be configured") {
		t.Errorf("unexpected error message: %v", err)
	}
}
