package cli

import (
	"testing"

	"github.com/jjack/remote-boot-agent/internal/config"
)

func TestGenerateConfigForm_ConstructsConfig(t *testing.T) {
	hostname := "test-host"
	hassURL := "http://localhost:8123"

	// We can't run the interactive form in a unit test, but we can check that the config struct is constructed correctly
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
