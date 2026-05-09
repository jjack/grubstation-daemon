package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	DefaultWolAddress = "255.255.255.255"
	DefaultWolPort    = 9
)

func DefaultConfigPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("AppData"), "grub-os-reporter", "config.yaml")
	}
	return "/etc/grub-os-reporter/config.yaml"
}

const (
	FlagGrubConfig  = "grub-config"
	FlagMac         = "host-mac"
	FlagName        = "host-name"
	FlagAddress     = "host-address"
	FlagWolAddress  = "wol-address"
	FlagWolPort     = "wol-port"
	FlagHassURL     = "homeassistant-url"
	FlagHassWebhook = "homeassistant-webhook-id"
	FlagDaemonPort  = "daemon-port"
	FlagDaemonKey   = "daemon-key"
)

var viperBindPFlag = func(v *viper.Viper, key string, flag *pflag.Flag) error { return v.BindPFlag(key, flag) }

type Config struct {
	Host          HostConfig          `mapstructure:"host"`
	WakeOnLan     WakeOnLanConfig     `mapstructure:"wake_on_lan"`
	HomeAssistant HomeAssistantConfig `mapstructure:"homeassistant"`
	Grub          GrubConfig          `mapstructure:"grub"`
	Daemon        DaemonConfig        `mapstructure:"daemon"`
}

type DaemonConfig struct {
	ListenPort        int    `mapstructure:"listen_port"`
	APIKey            string `mapstructure:"api_key"`
	ReportBootOptions bool   `mapstructure:"report_boot_options"`
}

type GrubConfig struct {
	ConfigPath      string `mapstructure:"config_path"`
	WaitTimeSeconds int    `mapstructure:"wait_time_seconds"`
}

type WakeOnLanConfig struct {
	Address string `mapstructure:"address"`
	Port    int    `mapstructure:"port"`
}

type HostConfig struct {
	Name       string `mapstructure:"name"`
	Address    string `mapstructure:"address"`
	MACAddress string `mapstructure:"mac"`
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
			"wake_on_lan.address":      FlagWolAddress,
			"wake_on_lan.port":         FlagWolPort,
			"homeassistant.url":        FlagHassURL,
			"homeassistant.webhook_id": FlagHassWebhook,
			"daemon.listen_port":       FlagDaemonPort,
			"daemon.api_key":           FlagDaemonKey,
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

	if cfg.Daemon.ListenPort == 0 {
		cfg.Daemon.ListenPort = 8081
	}
	if cfg.Grub.WaitTimeSeconds == 0 {
		cfg.Grub.WaitTimeSeconds = 2
	}

	return &cfg, nil
}

func Save(cfg *Config, path string) error {
	v := viper.New()
	v.Set("host.mac", cfg.Host.MACAddress)
	v.Set("host.name", cfg.Host.Name)
	v.Set("host.address", cfg.Host.Address)

	if cfg.WakeOnLan.Address != "" && cfg.WakeOnLan.Address != DefaultWolAddress {
		v.Set("wake_on_lan.address", cfg.WakeOnLan.Address)
	}
	if cfg.WakeOnLan.Port != 0 && cfg.WakeOnLan.Port != DefaultWolPort {
		v.Set("wake_on_lan.port", cfg.WakeOnLan.Port)
	}

	v.Set("homeassistant.url", cfg.HomeAssistant.URL)
	v.Set("homeassistant.webhook_id", cfg.HomeAssistant.WebhookID)
	v.Set("daemon.listen_port", cfg.Daemon.ListenPort)
	if cfg.Daemon.APIKey != "" {
		v.Set("daemon.api_key", cfg.Daemon.APIKey)
	}
	v.Set("daemon.report_boot_options", cfg.Daemon.ReportBootOptions)
	if cfg.Grub.ConfigPath != "" {
		v.Set("grub.config_path", cfg.Grub.ConfigPath)
	}
	if cfg.Grub.WaitTimeSeconds != 0 && cfg.Grub.WaitTimeSeconds != 2 {
		v.Set("grub.wait_time_seconds", cfg.Grub.WaitTimeSeconds)
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
