package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

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
	cli := &CLI{
		Config: &config.Config{
			Host: config.HostConfig{
				MACAddress: "aa:bb:cc:dd:ee:ff",
				Hostname:   "test-host",
			},
			Bootloader: config.BootloaderConfig{
				Name:       "grub",
				ConfigPath: tempGrubPath,
			},
			HomeAssistant: config.HomeAssistantConfig{
				URL:       ts.URL,
				WebhookID: "test-webhook",
			},
		},
	}

	cmd := NewPushBootOptions(cli)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if payload.MACAddress != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("expected MAC address aa:bb:cc:dd:ee:ff, got %s", payload.MACAddress)
	}
	if payload.Hostname != "test-host" {
		t.Errorf("expected hostname test-host, got %s", payload.Hostname)
	}
	if payload.Bootloader != "example" {
		t.Errorf("expected bootloader example, got %s", payload.Bootloader)
	}
	if len(payload.BootOptions) != 2 || payload.BootOptions[0] != "Ubuntu" || payload.BootOptions[1] != "Windows" {
		t.Errorf("expected [Ubuntu, Windows], got %v", payload.BootOptions)
	}
}

func TestPushBootOptionsCommand_MissingHAConfig(t *testing.T) {
	cli := &CLI{
		Config: &config.Config{
			Bootloader: config.BootloaderConfig{
				Name: "example",
			},
			HomeAssistant: config.HomeAssistantConfig{
				URL: "",
			},
		},
	}

	cmd := NewPushBootOptions(cli)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error due to missing HA config, got nil")
	}
	if !strings.Contains(err.Error(), "homeassistant url and webhook_id must be configured") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPushBootOptionsCommand_UnknownBootloader(t *testing.T) {
	cli := &CLI{
		Config: &config.Config{
			Bootloader: config.BootloaderConfig{
				Name: "unknown",
			},
		},
	}
	cmd := NewPushBootOptions(cli)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
}
