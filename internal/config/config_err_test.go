package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func TestLoad_NoFile(t *testing.T) {
	// Don't pass a file, ensure it attempts to find and ends up with defaults
	_, err := Load("", nil)
	if err != nil {
		t.Fatalf("expected no error when no file is present and not provided, got %v", err)
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

func TestLoad_UnmarshalError(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Write invalid type structure to cause unmarshal to fail
	configData := []byte(`host: "this is a string, not a struct"`)

	if err := os.WriteFile(configPath, configData, 0o644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	_, err := Load(configPath, nil)
	if err == nil {
		t.Fatal("expected error on unmarshal")
	}
	if !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSave_Error(t *testing.T) {
	cfg := &Config{}
	err := Save(cfg, "/nonexistent_dir_12345/config.yaml")
	if err == nil {
		t.Fatal("expected error saving to invalid path")
	}
}

func TestLoad_BindPFlagError(t *testing.T) {
	oldBindPFlag := viperBindPFlag
	defer func() { viperBindPFlag = oldBindPFlag }()

	viperBindPFlag = func(v *viper.Viper, key string, flag *pflag.Flag) error {
		return errors.New("mock bind error")
	}

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String(FlagWolAddress, "", "")

	_, err := Load("", fs)
	if err == nil || !strings.Contains(err.Error(), "mock bind error") {
		t.Fatalf("expected mock bind error, got %v", err)
	}
}
