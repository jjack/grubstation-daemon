package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
)

func TestConfig_SaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.yaml")

	cfg := &Config{
		Server: ServerConfig{
			MACAddress:       "00:11:22:33:44:55",
			Name:             "Test Server",
			Host:             "test-host",
			BroadcastAddress: "192.168.1.255",
			BroadcastPort:    9,
		},
		Bootloader: BootloaderConfig{
			Name:       "grub",
			ConfigPath: "/boot/grub/grub.cfg",
		},
		InitSystem: InitSystemConfig{
			Name: "systemd",
		},
		HomeAssistant: HomeAssistantConfig{
			URL:        "http://ha.local",
			WebhookID:  "test-webhook",
			EntityType: EntityTypeButton,
		},
	}

	// Test writing to the filesystem
	err := Save(cfg, cfgPath)
	if err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Fatalf("expected config file to exist at %s", cfgPath)
	}

	// Test loading from the filesystem
	loadedCfg, err := Load(cfgPath, nil)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loadedCfg.Server.MACAddress != cfg.Server.MACAddress {
		t.Errorf("expected MAC %s, got %s", cfg.Server.MACAddress, loadedCfg.Server.MACAddress)
	}
	if loadedCfg.Bootloader.ConfigPath != cfg.Bootloader.ConfigPath {
		t.Errorf("expected Bootloader path %s, got %s", cfg.Bootloader.ConfigPath, loadedCfg.Bootloader.ConfigPath)
	}
	if loadedCfg.HomeAssistant.WebhookID != cfg.HomeAssistant.WebhookID {
		t.Errorf("expected Webhook ID %s, got %s", cfg.HomeAssistant.WebhookID, loadedCfg.HomeAssistant.WebhookID)
	}
}

func TestConfig_SaveError(t *testing.T) {
	cfg := &Config{}
	// Passing a directory path should cause WriteConfigAs to fail
	err := Save(cfg, t.TempDir())
	if err == nil {
		t.Fatal("expected error when saving to a directory path, got nil")
	}
}

func TestConfig_LoadDefaults(t *testing.T) {
	originalWD, _ := os.Getwd()
	_ = os.Chdir(t.TempDir()) // Ensure we're in an empty directory without a config file
	defer func() { _ = os.Chdir(originalWD) }()

	cfg, err := Load("", nil)
	if err != nil {
		t.Fatalf("expected no error when config file is absent, got: %v", err)
	}
	if cfg == nil {
		t.Fatalf("expected a valid, empty config object, got nil")
	}
}

func TestLoad_WithFlags(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("mac", "aa:bb:cc:dd:ee:ff", "")
	fs.String("name", "flag-name", "")
	fs.String("host", "flag-host", "")
	fs.String("broadcast-address", "1.1.1.1", "")
	fs.Int("wol-port", 7, "")
	fs.String("bootloader", "grub-flag", "")
	fs.String("bootloader-path", "/flag/path", "")
	fs.String("init-system", "sysd-flag", "")
	fs.String("entity-type", "switch", "")
	fs.String("hass-url", "http://flag", "")
	fs.String("hass-webhook", "flag-webhook", "")

	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(""), 0o644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	cfg, err := Load(cfgPath, fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.MACAddress != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("expected mac aa:bb:cc:dd:ee:ff, got %v", cfg.Server.MACAddress)
	}
	if cfg.Server.Name != "flag-name" {
		t.Errorf("expected name flag-name, got %v", cfg.Server.Name)
	}
	if cfg.Server.Host != "flag-host" {
		t.Errorf("expected host flag-host, got %v", cfg.Server.Host)
	}
	if cfg.Server.BroadcastAddress != "1.1.1.1" {
		t.Errorf("expected broadcast address 1.1.1.1, got %v", cfg.Server.BroadcastAddress)
	}
	if cfg.Server.BroadcastPort != 7 {
		t.Errorf("expected wol-port 7, got %v", cfg.Server.BroadcastPort)
	}
	if cfg.Bootloader.Name != "grub-flag" {
		t.Errorf("expected bootloader name grub-flag, got %v", cfg.Bootloader.Name)
	}
	if cfg.Bootloader.ConfigPath != "/flag/path" {
		t.Errorf("expected bootloader path /flag/path, got %v", cfg.Bootloader.ConfigPath)
	}
	if cfg.InitSystem.Name != "sysd-flag" {
		t.Errorf("expected init system sysd-flag, got %v", cfg.InitSystem.Name)
	}
	if cfg.HomeAssistant.EntityType != "switch" {
		t.Errorf("expected entity type switch, got %v", cfg.HomeAssistant.EntityType)
	}
	if cfg.HomeAssistant.URL != "http://flag" {
		t.Errorf("expected url http://flag, got %v", cfg.HomeAssistant.URL)
	}
	if cfg.HomeAssistant.WebhookID != "flag-webhook" {
		t.Errorf("expected webhook flag-webhook, got %v", cfg.HomeAssistant.WebhookID)
	}
}
