package cli

import (
	"bytes"
	"os"
	"testing"
)

func TestCLI_PersistentPreRun(t *testing.T) {
	cli := NewCLI()

	// Empty config
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(f.Name()) }()

	_, _ = f.Write([]byte("host:\n  mac: test-mac\n  hostname: test-hostname\nbootloader:\n  name: example\n"))
	_ = f.Close()

	// Create a temporary grub config file to mock bootloader config detection
	tempGrubPath := createTempGrubConfig(t)

	cli.RootCmd.SetArgs([]string{
		"list",
		"--config", f.Name(),
		"--mac", "override-mac",
		"--hostname", "override-host",
		"--bootloader", "grub",
		"--bootloader-path", tempGrubPath,
		"--hass-url", "http://override-ha",
		"--hass-webhook", "override-webhook",
	})

	var b bytes.Buffer
	cli.RootCmd.SetOut(&b)

	// Since list with grub will try to parse the temp config, we can just let it fail if the logic is wrong.
	err = cli.Execute()
	// We expect no error for valid grub config
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all the overrides took effect in the config parsing layer
	if cli.Config.Host.MACAddress != "override-mac" {
		t.Errorf("mac not overridden")
	}
	if cli.Config.Host.Hostname != "override-host" {
		t.Errorf("host not overridden")
	}
	if cli.Config.Bootloader.Name != "grub" {
		t.Errorf("bl not overridden")
	}
	if cli.Config.Bootloader.ConfigPath != tempGrubPath {
		t.Errorf("bl cfg not overridden")
	}
	if cli.Config.HomeAssistant.URL != "http://override-ha" {
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

	// Create an empty temp file to simulate empty config
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	_ = f.Close()

	cli.RootCmd.SetArgs([]string{
		"list",
		"--config", f.Name(),
		"--bootloader", "grub",
		"--bootloader-path", tempGrubPath,
	})

	var b bytes.Buffer
	cli.RootCmd.SetOut(&b)

	// Since we mock example bootloader as active, it should succeed up to list print!
	// But List tries to auto-detect if config empty, "example" is active.
	err = cli.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
