package config

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config represents the loaded configuration from file or env vars
type Config struct {
	HomeAssistant HAConfig   `mapstructure:"homeassistant"`
	Host          HostConfig `mapstructure:"host"`
}

const DefaultWebhookID = "remote_boot_manager_ingest"

type HostConfig struct {
	Bootloader           string `mapstructure:"bootloader"`
	BootloaderConfigPath string `mapstructure:"bootloader_config_path"`
	MACAddress           string `mapstructure:"mac_address"`
	Hostname             string `mapstructure:"hostname"`
}

type HAConfig struct {
	BaseURL   string `mapstructure:"url"`
	WebhookID string `mapstructure:"webhook_id"`
}

func InitFlags(flags *pflag.FlagSet) {
	flags.String("config", "", "Explicit config file path (default is /etc/remote-boot-agent/config.yaml)")
	flags.String("bootloader", "", "Name of the bootloader to use (optional, will be auto-detected if not provided)")
	flags.String("bootloader-config-path", "", "Explicit path to the bootloader configuration file")
	flags.String("mac-address", "", "MAC address of the device (optional, will be auto-detected if not provided)")
	flags.String("hostname", "", "Hostname of the device (optional, will be auto-detected if not provided)")

	flags.String("homeassistant-url", "", "Home Assistant Base URL")
	flags.String("homeassistant-webhook-id", DefaultWebhookID, "Home Assistant Webhook ID")
}

// Load reads and parses configuration for the CLI application
func Load(flags *pflag.FlagSet) (*Config, error) {
	v := viper.New()

	bindings := map[string]string{
		"host.bootloader":             "bootloader",
		"host.bootloader_config_path": "bootloader-config-path",
		"host.mac_address":            "mac-address",
		"host.hostname":               "hostname",
		"homeassistant.url":           "homeassistant-url",
		"homeassistant.webhook_id":    "homeassistant-webhook-id",
	}

	for key, flag := range bindings {
		if err := v.BindPFlag(key, flags.Lookup(flag)); err != nil {
			return nil, fmt.Errorf("error binding %s flag: %w", flag, err)
		}
	}

	v.SetDefault("homeassistant.webhook_id", DefaultWebhookID)

	cfgFile, err := flags.GetString("config")
	if err != nil {
		return nil, fmt.Errorf("error reading config flag: %w", err)
	}

	if cfgFile != "" {
		// Use config file from the flag
		v.SetConfigFile(cfgFile)
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}

		// Search for config in common locations
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("/etc/remote-boot-agent/")
		v.AddConfigPath(fmt.Sprintf("%s/.config/remote-boot-agent/", homeDir))
		v.AddConfigPath(".")
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// File was found but contained errors
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// File not found; ignore and proceed with flags/defaults
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode into config struct: %w", err)
	}

	// Discover Hardware network info if missing
	if cfg.Host.Hostname == "" {
		host, err := DetectHostname()
		if err != nil {
			return nil, fmt.Errorf("failed to detect hostname: %w", err)
		}
		cfg.Host.Hostname = host
	}
	if cfg.Host.MACAddress == "" {
		mac, err := DetectMacAddress()
		if err != nil {
			return nil, fmt.Errorf("failed to detect MAC address: %w", err)
		}
		cfg.Host.MACAddress = mac
	}

	return &cfg, nil
}
