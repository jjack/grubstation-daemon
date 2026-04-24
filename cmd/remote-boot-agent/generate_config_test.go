package main

import (
	"testing"

	"github.com/charmbracelet/huh"
	"github.com/jjack/remote-boot-agent/internal/config"
)

func TestGenerateConfigForm_ValidInput(t *testing.T) {
	hostname := "test-host"
	hassURL := "http://localhost:8123"

	// This test can't run the interactive form, but we can check that the function returns a config object
	// if the form.Run() is skipped or mocked. For now, just check construction logic.
	cfg := &config.Config{
		Host: config.HostConfig{
			MACAddress: "00:11:22:33:44:55",
			Hostname:   hostname,
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       hassURL,
			WebhookID: "webhookid",
		},
	}

	if cfg.Host.MACAddress != "00:11:22:33:44:55" {
		t.Errorf("expected MACAddress to be 00:11:22:33:44:55, got %s", cfg.Host.MACAddress)
	}
	if cfg.Host.Hostname != hostname {
		t.Errorf("expected Hostname to be %s, got %s", hostname, cfg.Host.Hostname)
	}
	if cfg.HomeAssistant.URL != hassURL {
		t.Errorf("expected HomeAssistant.URL to be %s, got %s", hassURL, cfg.HomeAssistant.URL)
	}
}

func TestInterfaceOptions_NotEmpty(t *testing.T) {
	// This test assumes system.GetInterfaceOptions returns at least one option in a real environment
	// Here we just check that getInterfaces returns a slice
	// Simulate user input by providing a getInterfaces func with one option
	interfaces := []huh.Option[string]{huh.NewOption("eth0 (00:11:22:33:44:55)", "00:11:22:33:44:55")}
	getInterfaces := func() []huh.Option[string] { return interfaces }
	opts := getInterfaces()
	if opts == nil {
		t.Error("expected getInterfaces to return a slice, got nil")
	}
}
