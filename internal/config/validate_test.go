package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateBootloaderConfigPath(t *testing.T) {
	tempDir := t.TempDir()
	validPath := filepath.Join(tempDir, "grub.cfg")
	if err := os.WriteFile(validPath, []byte(""), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid path", validPath, false},
		{"empty path", "", true},
		{"not exist", filepath.Join(tempDir, "missing.cfg"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBootloaderConfigPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBootloaderConfigPath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateMACAddress(t *testing.T) {
	tests := []struct {
		name    string
		mac     string
		wantErr bool
	}{
		{"valid mac", "00:11:22:33:44:55", false},
		{"empty mac", "", true},
		{"invalid format", "invalid-mac", true},
		{"missing colons", "001122334455", false},
		{"too long", "00:11:22:33:44:55:66:77:88", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMACAddress(tt.mac)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMACAddress() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateHost(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		wantErr bool
	}{
		{"valid hostname", "my-host.name", false},
		{"valid ip", "192.168.1.5", false},
		{"empty hostname", "", true},
		{"invalid characters", "my_host!name", true},
		{"spaces", "my host", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHost(tt.host)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHost() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid http", "http://localhost:8123", false},
		{"invalid https", "https://homeassistant.local", true},
		{"empty", "", true},
		{"invalid format", "not-a-url", true},
		{"missing scheme", "/just/a/path", true},
		{"missing host", "http:///path", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateURL(tt.url); (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateWebhookID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid id", "my-webhook_123", false},
		{"empty", "", true},
		{"invalid characters", "webhook!", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateWebhookID(tt.id); (err != nil) != tt.wantErr {
				t.Errorf("ValidateWebhookID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateBroadcastAddress(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{"valid ip", "192.168.1.255", false},
		{"empty", "", false},
		{"invalid ip", "not-an-ip", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateBroadcastAddress(tt.addr); (err != nil) != tt.wantErr {
				t.Errorf("ValidateBroadcastAddress() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateBroadcastPort(t *testing.T) {
	tests := []struct {
		name    string
		port    string
		wantErr bool
	}{
		{"valid port", "9", false},
		{"empty", "", false},
		{"too low", "0", true},
		{"too high", "65536", true},
		{"not a number", "abc", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateBroadcastPort(tt.port); (err != nil) != tt.wantErr {
				t.Errorf("ValidateBroadcastPort() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	validCfg := func() *Config {
		return &Config{
			Host: HostConfig{
				MACAddress:       "00:11:22:33:44:55",
				Name:             "test-name",
				Address:          "test-host",
				BroadcastAddress: "192.168.1.255",
				BroadcastPort:    9,
			},
			HomeAssistant: HomeAssistantConfig{
				URL:       "http://localhost:8123",
				WebhookID: "test_webhook",
			},
		}
	}

	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{"valid config", func(c *Config) {}, false},
		{"invalid MAC", func(c *Config) { c.Host.MACAddress = "invalid" }, true},
		{"empty Name", func(c *Config) { c.Host.Name = "" }, true},
		{"invalid Address", func(c *Config) { c.Host.Address = "invalid address format" }, true},
		{"empty URL", func(c *Config) { c.HomeAssistant.URL = "" }, true},
		{"empty WebhookID", func(c *Config) { c.HomeAssistant.WebhookID = "" }, true},
		{"invalid BroadcastPort", func(c *Config) { c.Host.BroadcastPort = -1 }, true},
		{"invalid BroadcastAddress", func(c *Config) { c.Host.BroadcastAddress = "invalid-ip" }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validCfg()
			tt.modify(cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
