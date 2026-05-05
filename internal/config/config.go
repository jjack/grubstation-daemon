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

type Config struct {
	Host          HostConfig          `mapstructure:"host"`
	Bootloader    BootloaderConfig    `mapstructure:"bootloader"`
	InitSystem    InitSystemConfig    `mapstructure:"initsystem"`
	HomeAssistant HomeAssistantConfig `mapstructure:"homeassistant"`
}

type BootloaderConfig struct {
	Name       string `mapstructure:"name"`
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

	v.SetDefault("host.broadcast_address", DefaultBroadcastAddress)
	v.SetDefault("host.broadcast_port", DefaultBroadcastPort)

	if flags != nil {
		_ = v.BindPFlag("host.mac", flags.Lookup("mac"))
		_ = v.BindPFlag("host.name", flags.Lookup("name"))
		_ = v.BindPFlag("host.address", flags.Lookup("address"))
		_ = v.BindPFlag("host.broadcast_address", flags.Lookup("broadcast-address"))
		_ = v.BindPFlag("host.broadcast_port", flags.Lookup("broadcast-port"))
		_ = v.BindPFlag("bootloader.name", flags.Lookup("bootloader"))
		_ = v.BindPFlag("bootloader.config_path", flags.Lookup("bootloader-path"))
		_ = v.BindPFlag("initsystem.name", flags.Lookup("init-system"))
		_ = v.BindPFlag("homeassistant.url", flags.Lookup("hass-url"))
		_ = v.BindPFlag("homeassistant.webhook_id", flags.Lookup("hass-webhook"))
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

	v.Set("bootloader.name", cfg.Bootloader.Name)
	v.Set("bootloader.config_path", cfg.Bootloader.ConfigPath)
	v.Set("initsystem.name", cfg.InitSystem.Name)
	v.Set("homeassistant.url", cfg.HomeAssistant.URL)
	v.Set("homeassistant.webhook_id", cfg.HomeAssistant.WebhookID)

	if err := v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}
