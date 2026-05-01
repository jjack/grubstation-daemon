package config

import (
	"os"
	"path/filepath"
	"testing"
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
