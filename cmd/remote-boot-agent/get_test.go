package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jjack/remote-boot-agent/internal/config"
)

func TestGetSelectedOSCommand(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Ubuntu"))
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

	cmd := GetSelectedOS(cli)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.Execute()

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out, _ := io.ReadAll(r)
	output := string(out)

	if !strings.Contains(output, "Ubuntu") {
		t.Errorf("output missing selected OS name Ubuntu: %s", output)
	}
}

func TestGetSelectedOSCommand_MissingHAConfig(t *testing.T) {
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

	cmd := GetSelectedOS(cli)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error due to missing HA config, got nil")
	}
	if !strings.Contains(err.Error(), "homeassistant url not configured") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetSelectedOSCommand_UnknownBootloader(t *testing.T) {
	cli := &CLI{
		Config: &config.Config{
			Bootloader: config.BootloaderConfig{
				Name: "unknown",
			},
		},
	}
	cmd := GetSelectedOS(cli)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetSelectedOSCommand_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer ts.Close()

	cli := &CLI{
		Config: &config.Config{
			Bootloader: config.BootloaderConfig{
				Name: "example",
			},
			HomeAssistant: config.HomeAssistantConfig{
				URL:       ts.URL,
				WebhookID: "test-webhook",
			},
		},
	}
	cmd := GetSelectedOS(cli)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
}
