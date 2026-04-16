package main

import (
	"bytes"
	"os"
	"strings"
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

	_, _ = f.Write([]byte("host:\n  mac_address: test-mac\n  hostname: test-hostname\nbootloader:\n  name: example\n"))
	_ = f.Close()

	cli.RootCmd.SetArgs([]string{
		"list",
		"--config", f.Name(),
		"--mac", "override-mac",
		"--hostname", "override-host",
		"--bootloader", "override-bl",
		"--bootloader-config", "override-bl-cfg",
		"--ha-url", "http://override-ha",
		"--ha-webhook", "override-webhook",
	})

	var b bytes.Buffer
	cli.RootCmd.SetOut(&b)

	// Since list with override-bl will fail resolution during the command, we can just let it fail.
	// But it will process pre-run successfully!
	err = cli.Execute()

	if err == nil {
		t.Fatal("expected error due to invalid override bootloader")
	}

	if !strings.Contains(err.Error(), "specified bootloader override-bl not supported") {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify all the overrides took effect in the config parsing layer
	if cli.Config.Host.MACAddress != "override-mac" {
		t.Errorf("mac not overridden")
	}
	if cli.Config.Host.Hostname != "override-host" {
		t.Errorf("host not overridden")
	}
	if cli.Config.Bootloader.Name != "override-bl" {
		t.Errorf("bl not overridden")
	}
	if cli.Config.Bootloader.ConfigPath != "override-bl-cfg" {
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
	})

	var b bytes.Buffer
	cli.RootCmd.SetOut(&b)

	// Since we mock example bootloader as active, it should succeed up to list print!
	// But List tries to auto-detect if config empty, "example" is active.
	err = cli.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cli.Config.Host.MACAddress == "" {
		t.Errorf("expected MAC address to be auto-detected")
	}
	if cli.Config.Host.Hostname == "" {
		t.Errorf("expected hostname to be auto-detected")
	}
}
