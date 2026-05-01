package config

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type EntityType string

const (
	EntityTypeButton EntityType = "button"
	EntityTypeSwitch EntityType = "switch"
)

type Config struct {
	Server        ServerConfig        `mapstructure:"server"`
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

type ServerConfig struct {
	Host             string `mapstructure:"host"`
	MACAddress       string `mapstructure:"mac_address"`
	Name             string `mapstructure:"name"`
	BroadcastAddress string `mapstructure:"broadcast_address"`
	BroadcastPort    int    `mapstructure:"broadcast_port"`
}

type HomeAssistantConfig struct {
	EntityType EntityType `mapstructure:"entity_type"`
	URL        string     `mapstructure:"url"`
	WebhookID  string     `mapstructure:"webhook_id"`
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
		_ = v.BindPFlag("server.mac_address", flags.Lookup("mac"))
		_ = v.BindPFlag("server.name", flags.Lookup("name"))
		_ = v.BindPFlag("server.host", flags.Lookup("host"))
		_ = v.BindPFlag("server.broadcast_address", flags.Lookup("broadcast-address"))
		_ = v.BindPFlag("server.broadcast_port", flags.Lookup("wol-port"))
		_ = v.BindPFlag("host.mac_address", flags.Lookup("mac"))
		_ = v.BindPFlag("host.name", flags.Lookup("name"))
		_ = v.BindPFlag("host.host", flags.Lookup("host"))
		_ = v.BindPFlag("host.broadcast_address", flags.Lookup("broadcast-address"))
		_ = v.BindPFlag("host.broadcast_port", flags.Lookup("wol-port"))
		_ = v.BindPFlag("bootloader.name", flags.Lookup("bootloader"))
		_ = v.BindPFlag("bootloader.config_path", flags.Lookup("bootloader-path"))
		_ = v.BindPFlag("initsystem.name", flags.Lookup("init-system"))
		_ = v.BindPFlag("homeassistant.entity_type", flags.Lookup("entity-type"))
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
	v.Set("server.mac_address", cfg.Server.MACAddress)
	v.Set("server.name", cfg.Server.Name)
	v.Set("server.host", cfg.Server.Host)
	v.Set("server.broadcast_address", cfg.Server.BroadcastAddress)
	v.Set("server.broadcast_port", cfg.Server.BroadcastPort)
	v.Set("host.mac_address", cfg.Server.MACAddress)
	v.Set("host.name", cfg.Server.Name)
	v.Set("host.host", cfg.Server.Host)
	v.Set("host.broadcast_address", cfg.Server.BroadcastAddress)
	v.Set("host.broadcast_port", cfg.Server.BroadcastPort)
	v.Set("bootloader.name", cfg.Bootloader.Name)
	v.Set("bootloader.config_path", cfg.Bootloader.ConfigPath)
	v.Set("initsystem.name", cfg.InitSystem.Name)
	v.Set("homeassistant.entity_type", cfg.HomeAssistant.EntityType)
	v.Set("homeassistant.url", cfg.HomeAssistant.URL)
	v.Set("homeassistant.webhook_id", cfg.HomeAssistant.WebhookID)

	if err := v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}
