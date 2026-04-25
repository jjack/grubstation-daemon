package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_NoFile(t *testing.T) {
	// Don't pass a file, ensure it attempts to find and ends up with defaults
	cfg, err := Load("", nil)
	if err != nil {
		t.Fatalf("expected no error when no file is present and not provided, got %v", err)
	}
	if cfg.HomeAssistant.URL == "http://homeassistant.local:8123" {
		t.Errorf("expected default HA URL, got %s", cfg.HomeAssistant.URL)
	}
}

func TestLoad_InvalidFormat(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Write weird content to trigger Unmarshal or ReadInConfig failure
	configData := []byte(`\x00\x00\x00 invalid yaml`)

	if err := os.WriteFile(configPath, configData, 0o644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	_, err := Load(configPath, nil)
	if err == nil {
		t.Fatal("expected error on invalid format")
	}
	if !strings.Contains(err.Error(), "failed to read config") && !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("unexpected error message: %v", err)
	}
}
