package cli

import (
	"bytes"
	"testing"

	"github.com/jjack/remote-boot-agent/internal/config"
)

func TestCLI_PersistentPreRun(t *testing.T) {
	cli := NewCLI()

	// Create a temporary grub config file to mock bootloader config detection
	tempGrubPath := createTempGrubConfig(t)

	cli.RootCmd.SetArgs([]string{
		"options",
		"list",
		"--config", "../../config.sample.yaml",
		"--mac", "aa:bb:cc:dd:ee:ff",
		"--name", "override-name",
		"--host", "override-host",
		"--entity-type", "switch",
		"--broadcast-address", "192.168.1.255",
		"--wol-port", "7",
		"--bootloader", "grub",
		"--bootloader-path", tempGrubPath,
		"--init-system", "systemd",
		"--hass-url", "http://override-ha.local",
		"--hass-webhook", "override-webhook",
	})

	var b bytes.Buffer
	cli.RootCmd.SetOut(&b)

	// Since list with grub will try to parse the temp bootloader config, we can just let it fail if the logic is wrong.
	err := cli.Execute()
	// We expect no error for valid grub config
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all the overrides took effect in the config parsing layer
	if cli.Config.Server.MACAddress != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("mac not overridden")
	}
	if cli.Config.Server.Name != "override-name" {
		t.Errorf("name not overridden")
	}
	if cli.Config.Server.Host != "override-host" {
		t.Errorf("host not overridden")
	}
	if cli.Config.HomeAssistant.EntityType != config.EntityTypeSwitch {
		t.Errorf("entity type not overridden")
	}
	if cli.Config.Server.BroadcastAddress != "192.168.1.255" {
		t.Errorf("broadcast address not overridden")
	}
	if cli.Config.Server.BroadcastPort != 7 {
		t.Errorf("wol port not overridden")
	}
	if cli.Config.Bootloader.Name != "grub" {
		t.Errorf("bl not overridden")
	}
	if cli.Config.Bootloader.ConfigPath != tempGrubPath {
		t.Errorf("bl cfg not overridden")
	}
	if cli.Config.InitSystem.Name != "systemd" {
		t.Errorf("init system not overridden")
	}
	if cli.Config.HomeAssistant.URL != "http://override-ha.local" {
		t.Errorf("url not overridden")
	}
	if cli.Config.HomeAssistant.WebhookID != "override-webhook" {
		t.Errorf("webhook not overridden")
	}
}

func TestCLI_PersistentPreRun_ConfigLoadFail(t *testing.T) {
	// Create a temporary grub config file to mock bootloader config detection
	tempGrubPath := createTempGrubConfig(t)
	cli := NewCLI()

	cli.RootCmd.SetArgs([]string{
		"options",
		"list",
		"--config", "does-not-exist.yaml",
		"--bootloader", "grub",
		"--bootloader-path", tempGrubPath,
		"--mac", "00:11:22:33:44:55",
		"--name", "test-name",
		"--host", "test-host",
		"--entity-type", "button",
		"--hass-url", "http://test-ha.local",
		"--hass-webhook", "test-webhook",
	})

	var b bytes.Buffer
	cli.RootCmd.SetOut(&b)

	// Since we mock example bootloader as active, it should succeed up to list print!
	// But List tries to auto-detect if config empty, "example" is active.
	err := cli.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
