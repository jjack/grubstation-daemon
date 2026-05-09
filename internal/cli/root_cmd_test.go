package cli

import (
	"bytes"
	"testing"
)

func TestCLI_PersistentPreRun(t *testing.T) {
	cli := NewCLI()

	cli.RootCmd.SetArgs([]string{
		"config",
		"validate",
		"--config", "../../config.sample.yaml",
		"--grub-config", "/custom/grub.cfg",
		"--host-mac", "aa:bb:cc:dd:ee:ff",
		"--host-name", "override-name",
		"--host-address", "10.0.0.1",
		"--wol-address", "192.168.1.255",
		"--wol-port", "7",
		"--homeassistant-url", "http://override-ha.local",
		"--homeassistant-webhook-id", "override-webhook",
	})

	var b bytes.Buffer
	cli.RootCmd.SetOut(&b)

	err := cli.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all the overrides took effect in the config parsing layer
	if cli.Config.Grub.ConfigPath != "/custom/grub.cfg" {
		t.Errorf("grub config not overridden")
	}
	if cli.Config.Host.MACAddress != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("mac not overridden")
	}
	if cli.Config.Host.Name != "override-name" {
		t.Errorf("name not overridden")
	}
	if cli.Config.Host.Address != "10.0.0.1" {
		t.Errorf("address not overridden")
	}
	if cli.Config.WakeOnLan.Address != "192.168.1.255" {
		t.Errorf("broadcast address not overridden")
	}
	if cli.Config.WakeOnLan.Port != 7 {
		t.Errorf("wol port not overridden")
	}
	if cli.Config.HomeAssistant.URL != "http://override-ha.local" {
		t.Errorf("url not overridden")
	}
	if cli.Config.HomeAssistant.WebhookID != "override-webhook" {
		t.Errorf("webhook not overridden")
	}
}

func TestCLI_PersistentPreRun_ConfigLoadFail(t *testing.T) {
	cli := NewCLI()

	cli.RootCmd.SetArgs([]string{
		"config",
		"validate",
		"--config", "does-not-exist.yaml",
		"--host-mac", "00:11:22:33:44:55",
		"--host-name", "test-name",
		"--host-address", "test-host",
		"--homeassistant-url", "http://test-ha.local",
		"--homeassistant-webhook-id", "test-webhook",
	})

	var b bytes.Buffer
	cli.RootCmd.SetOut(&b)

	err := cli.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
