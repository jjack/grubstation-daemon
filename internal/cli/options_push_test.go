package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
	ha "github.com/jjack/remote-boot-agent/internal/homeassistant"
)

// createTempGrubConfig creates a temporary grub config file and returns its path and a cleanup function.
func createTempGrubConfig(t *testing.T) string {
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
	t.Parallel()

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

	tempGrubPath := createTempGrubConfig(t)
	cfg := &config.Config{
		Host: config.HostConfig{
			MACAddress:       "aa:bb:cc:dd:ee:ff",
			BroadcastAddress: "192.168.1.255",
			BroadcastPort:    7, // use a custom port to ensure it gets passed
			Name:             "test-name",
			Address:          "10.0.0.1",
		},
		Bootloader: config.BootloaderConfig{
			Name:       "grub",
			ConfigPath: tempGrubPath,
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       ts.URL,
			WebhookID: "test-webhook",
		},
	}

	registry := bootloader.NewRegistry()
	registry.Register("grub", bootloader.NewGrub)

	deps := &CommandDeps{Config: cfg, BootloaderRegistry: registry}
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
	if payload.Bootloader != "grub" {
		t.Errorf("expected bootloader grub, got %s", payload.Bootloader)
	}
	if len(payload.BootOptions) != 1 || payload.BootOptions[0] != "Test OS" {
		t.Errorf("expected [Test OS], got %v", payload.BootOptions)
	}
}

func TestPushBootOptionsCommand_DefaultWOL(t *testing.T) {
	t.Parallel()

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

	tempGrubPath := createTempGrubConfig(t)
	cfg := &config.Config{
		Host: config.HostConfig{
			MACAddress:       "aa:bb:cc:dd:ee:ff",
			BroadcastAddress: config.DefaultBroadcastAddress,
			BroadcastPort:    config.DefaultBroadcastPort,
			Name:             "test-name",
			Address:          "10.0.0.1",
		},
		Bootloader: config.BootloaderConfig{
			Name:       "grub",
			ConfigPath: tempGrubPath,
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       ts.URL,
			WebhookID: "test-webhook",
		},
	}

	registry := bootloader.NewRegistry()
	registry.Register("grub", bootloader.NewGrub)

	deps := &CommandDeps{Config: cfg, BootloaderRegistry: registry}
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

type mockPushBootloaderErr struct{}

func (m *mockPushBootloaderErr) Name() string                      { return "err" }
func (m *mockPushBootloaderErr) IsActive(ctx context.Context) bool { return true }
func (m *mockPushBootloaderErr) GetBootOptions(ctx context.Context, cfg bootloader.Config) ([]string, error) {
	return nil, errors.New("mock error")
}

func (m *mockPushBootloaderErr) Install(ctx context.Context, macAddress, haURL, webhookID string) error {
	return nil
}

func (m *mockPushBootloaderErr) DiscoverConfigPath(ctx context.Context) (string, error) {
	return "", nil
}

func TestPushBootOptionsCommand_BootloaderError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Bootloader: config.BootloaderConfig{
			Name: "err",
		},
	}

	registry := bootloader.NewRegistry()
	registry.Register("err", func() bootloader.Bootloader { return &mockPushBootloaderErr{} })

	deps := &CommandDeps{Config: cfg, BootloaderRegistry: registry}
	cmd := NewPushCmd(deps)
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error from GetBootOptions, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get boot options") {
		t.Errorf("unexpected error message: %v", err)
	}
}

type mockPushBootloader struct{}

func (m *mockPushBootloader) Name() string                      { return "mock" }
func (m *mockPushBootloader) IsActive(ctx context.Context) bool { return true }
func (m *mockPushBootloader) GetBootOptions(ctx context.Context, cfg bootloader.Config) ([]string, error) {
	return []string{"OS 1"}, nil
}

func (m *mockPushBootloader) Install(ctx context.Context, macAddress, haURL, webhookID string) error {
	return nil
}
func (m *mockPushBootloader) DiscoverConfigPath(ctx context.Context) (string, error) { return "", nil }

func TestPushBootOptionsCommand_HAClientError(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cfg := &config.Config{
		Bootloader:    config.BootloaderConfig{Name: "mock"},
		HomeAssistant: config.HomeAssistantConfig{URL: ts.URL, WebhookID: "test"},
	}
	registry := bootloader.NewRegistry()
	registry.Register("mock", func() bootloader.Bootloader { return &mockPushBootloader{} })

	deps := &CommandDeps{Config: cfg, BootloaderRegistry: registry}
	cmd := NewPushCmd(deps)
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error from HA Push, got nil")
	}
}

func TestPushBootOptionsCommand_MissingHAConfig(t *testing.T) {
	t.Parallel()

	tempGrubPath := createTempGrubConfig(t)
	cfg := &config.Config{
		Bootloader: config.BootloaderConfig{
			Name:       "grub",
			ConfigPath: tempGrubPath,
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL: "",
		},
	}

	registry := bootloader.NewRegistry()
	registry.Register("grub", bootloader.NewGrub)

	deps := &CommandDeps{Config: cfg, BootloaderRegistry: registry}
	cmd := NewPushCmd(deps)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error due to missing HA config, got nil")
	}
	if !strings.Contains(err.Error(), "homeassistant url and webhook_id must be configured") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPushBootOptionsCommand_UnknownBootloader(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Bootloader: config.BootloaderConfig{
			Name: "unknown",
		},
	}

	registry := bootloader.NewRegistry()

	deps := &CommandDeps{Config: cfg, BootloaderRegistry: registry}
	cmd := NewPushCmd(deps)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
}
