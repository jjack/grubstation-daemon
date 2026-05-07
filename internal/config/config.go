package config

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	DefaultBroadcastAddress = "255.255.255.255"
	DefaultBroadcastPort    = 9
)

const (
	FlagGrubConfig       = "grub-config"
	FlagMac              = "mac"
	FlagName             = "name"
	FlagAddress          = "address"
	FlagBroadcastAddress = "broadcast-address"
	FlagBroadcastPort    = "broadcast-port"
	FlagInitSystem       = "init-system"
	FlagHassURL          = "hass-url"
	FlagHassWebhook      = "hass-webhook"
)

var viperBindPFlag = func(v *viper.Viper, key string, flag *pflag.Flag) error { return v.BindPFlag(key, flag) }

type Config struct {
	Host          HostConfig          `mapstructure:"host"`
	InitSystem    InitSystemConfig    `mapstructure:"initsystem"`
	HomeAssistant HomeAssistantConfig `mapstructure:"homeassistant"`
	Grub          GrubConfig          `mapstructure:"grub"`
}

type GrubConfig struct {
	ConfigPath string `mapstructure:"config_path"`
}

type InitSystemConfig struct {
	Name string `mapstructure:"name"`
}

type HostConfig struct {
	Name             string `mapstructure:"name"`
	Address          string `mapstructure:"address"`
	MACAddress       string `mapstructure:"mac"`
	BroadcastAddress string `mapstructure:"broadcast_address"`
	BroadcastPort    int    `mapstructure:"broadcast_port"`
}

type HomeAssistantConfig struct {
	URL       string `mapstructure:"url"`
	WebhookID string `mapstructure:"webhook_id"`
}

func Load(cfgFile string, flags *pflag.FlagSet) (*Config, error) {
	v := viper.New()
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.AddConfigPath(".")
		v.SetConfigName("config")
		v.SetConfigType("yaml")
	}

	if flags != nil {
		flagMap := map[string]string{
			"grub.config_path":         FlagGrubConfig,
			"host.mac":                 FlagMac,
			"host.name":                FlagName,
			"host.address":             FlagAddress,
			"host.broadcast_address":   FlagBroadcastAddress,
			"host.broadcast_port":      FlagBroadcastPort,
			"initsystem.name":          FlagInitSystem,
			"homeassistant.url":        FlagHassURL,
			"homeassistant.webhook_id": FlagHassWebhook,
		}
		for configKey, flagName := range flagMap {
			if flag := flags.Lookup(flagName); flag != nil {
				if err := viperBindPFlag(v, configKey, flag); err != nil {
					return nil, fmt.Errorf("failed to bind flag %s: %w", flagName, err)
				}
			}
		}
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	return &cfg, nil
}

func Save(cfg *Config, path string) error {
	v := viper.New()
	v.Set("host.mac", cfg.Host.MACAddress)
	v.Set("host.name", cfg.Host.Name)
	v.Set("host.address", cfg.Host.Address)

	if cfg.Host.BroadcastAddress != "" && cfg.Host.BroadcastAddress != DefaultBroadcastAddress {
		v.Set("host.broadcast_address", cfg.Host.BroadcastAddress)
	}
	if cfg.Host.BroadcastPort != 0 && cfg.Host.BroadcastPort != DefaultBroadcastPort {
		v.Set("host.broadcast_port", cfg.Host.BroadcastPort)
	}

	v.Set("initsystem.name", cfg.InitSystem.Name)
	v.Set("homeassistant.url", cfg.HomeAssistant.URL)
	v.Set("homeassistant.webhook_id", cfg.HomeAssistant.WebhookID)
	if cfg.Grub.ConfigPath != "" {
		v.Set("grub.config_path", cfg.Grub.ConfigPath)
	}

	if err := v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Secure the config file to prevent unprivileged users from reading the Home Assistant webhook secret.
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("failed to secure config file permissions: %w", err)
	}
	return nil
}
