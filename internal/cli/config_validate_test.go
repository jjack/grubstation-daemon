package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jjack/grub-os-reporter/internal/config"
)

func TestConfigValidateCmd_Valid(t *testing.T) {
	cfg := &config.Config{
		Host: config.HostConfig{
			MACAddress:       "00:11:22:33:44:55",
			Name:             "test-name",
			Address:          "test-host",
			BroadcastAddress: "192.168.1.255",
			BroadcastPort:    9,
		},
		InitSystem: config.InitSystemConfig{
			Name: "systemd",
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       "http://ha.local",
			WebhookID: "test-webhook",
		},
	}

	deps := &CommandDeps{Config: cfg}
	cmd := NewConfigValidateCmd(deps)

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "Configuration is valid.") {
		t.Errorf("expected output to contain 'Configuration is valid.', got %s", out.String())
	}
}

func TestConfigValidateCmd_Invalid(t *testing.T) {
	// An empty config should fail validation
	cfg := &config.Config{}
	deps := &CommandDeps{Config: cfg}
	cmd := NewConfigValidateCmd(deps)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid config, got nil")
	}

	if !strings.Contains(err.Error(), "configuration is invalid") {
		t.Errorf("expected error to contain 'configuration is invalid', got %v", err)
	}
}
