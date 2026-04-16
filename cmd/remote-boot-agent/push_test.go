package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jjack/remote-boot-agent/internal/config"
	ha "github.com/jjack/remote-boot-agent/internal/homeassistant"
)

func TestPushOSesCommand(t *testing.T) {
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

	cli := &CLI{
		Config: &config.Config{
			Host: config.HostConfig{
				MACAddress: "aa:bb:cc:dd:ee:ff",
				Hostname:   "test-host",
			},
			Bootloader: config.BootloaderConfig{
				Name: "example",
			},
			HomeAssistant: config.HomeAssistantConfig{
				URL:       ts.URL,
				WebhookID: "test-webhook",
			},
		},
	}

	cmd := PushOSes(cli)
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
	if len(payload.OSList) != 2 || payload.OSList[0] != "Ubuntu" || payload.OSList[1] != "Windows" {
		t.Errorf("expected [Ubuntu, Windows], got %v", payload.OSList)
	}
}

func TestPushOSesCommand_MissingHAConfig(t *testing.T) {
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

	cmd := PushOSes(cli)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error due to missing HA config, got nil")
	}
	if !strings.Contains(err.Error(), "homeassistant url and webhook_id must be configured") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPushOSesCommand_UnknownBootloader(t *testing.T) {
	cli := &CLI{
		Config: &config.Config{
			Bootloader: config.BootloaderConfig{
				Name: "unknown",
			},
		},
	}
	cmd := PushOSes(cli)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
}
