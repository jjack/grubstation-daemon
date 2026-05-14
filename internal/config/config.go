package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const (
	DefaultWolBroadcastAddress = "255.255.255.255"
	DefaultWolBroadcastPort    = 9
	DefaultAgentPort           = 8081
	DefaultGrubWaitSeconds     = 2
)

func DefaultConfigPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("AppData"), "grubstation", "config.yaml")
	}
	return "/etc/grubstation/config.yaml"
}

const (
	FlagGrubConfig          = "grub-config"
	FlagMac                 = "host-mac"
	FlagAddress             = "host-address"
	FlagWolBroadcastAddress = "broadcast-address"
	FlagWolBroadcastPort    = "broadcast-port"
	FlagHassURL             = "homeassistant-url"
	FlagHassWebhook         = "homeassistant-webhook-id"
	FlagAgentPort           = "daemon-port"
	FlagDaemonKey           = "daemon-key"
)

var viperBindPFlag = func(v *viper.Viper, key string, flag *pflag.Flag) error { return v.BindPFlag(key, flag) }

type Config struct {
	Host          HostConfig          `yaml:"host"`
	WakeOnLan     *WakeOnLanConfig    `yaml:"wake_on_lan,omitempty"`
	HomeAssistant HomeAssistantConfig `yaml:"homeassistant"`
	Grub          *GrubConfig         `yaml:"grub,omitempty"`
	Daemon        DaemonConfig        `yaml:"daemon"`
}

type DaemonConfig struct {
	Port              int    `yaml:"port"`
	APIKey            string `yaml:"api_key,omitempty"`
	ReportBootOptions bool   `yaml:"report_boot_options"`
}

type GrubConfig struct {
	ConfigPath      string `yaml:"config_path,omitempty"`
	WaitTimeSeconds int    `yaml:"wait_time_seconds,omitempty"`
	URL             string `yaml:"url,omitempty"`
}

type WakeOnLanConfig struct {
	Address string `yaml:"address,omitempty"`
	Port    int    `yaml:"port,omitempty"`
}

type HostConfig struct {
	Address    string `yaml:"address"`
	MACAddress string `yaml:"mac"`
}

type HomeAssistantConfig struct {
	URL       string `yaml:"url"`
	WebhookID string `yaml:"webhook_id"`
}

func (c *Config) ToYAML(maskWebhook bool) (string, error) {
	displayCfg := *c

	// If sub-configs are empty or default, nil them out so omitempty works
	if displayCfg.WakeOnLan != nil {
		if displayCfg.WakeOnLan.Address == DefaultWolBroadcastAddress {
			displayCfg.WakeOnLan.Address = ""
		}
		if displayCfg.WakeOnLan.Port == DefaultWolBroadcastPort {
			displayCfg.WakeOnLan.Port = 0
		}
		if displayCfg.WakeOnLan.Address == "" && displayCfg.WakeOnLan.Port == 0 {
			displayCfg.WakeOnLan = nil
		}
	}
	if displayCfg.Grub != nil {
		if displayCfg.Grub.WaitTimeSeconds == DefaultGrubWaitSeconds {
			displayCfg.Grub.WaitTimeSeconds = 0
		}
		if displayCfg.Grub.WaitTimeSeconds == 0 && displayCfg.Grub.ConfigPath == "" && displayCfg.Grub.URL == "" {
			displayCfg.Grub = nil
		}
	}

	if maskWebhook {
		displayCfg.HomeAssistant.WebhookID = displayCfg.HomeAssistant.WebhookID[:4] + "..." + displayCfg.HomeAssistant.WebhookID[len(displayCfg.HomeAssistant.WebhookID)-4:]
	}

	out, err := yaml.Marshal(displayCfg)
	if err != nil {
		return "", err
	}

	return string(out), nil
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
			"host.address":             FlagAddress,
			"wake_on_lan.address":      FlagWolBroadcastAddress,
			"wake_on_lan.port":         FlagWolBroadcastPort,
			"homeassistant.url":        FlagHassURL,
			"homeassistant.webhook_id": FlagHassWebhook,
			"daemon.port":              FlagAgentPort,
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
	// Use "yaml" tags for unmarshaling to avoid redundancy
	if err := v.Unmarshal(&cfg, func(dc *mapstructure.DecoderConfig) {
		dc.TagName = "yaml"
	}); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	if cfg.Daemon.Port == 0 {
		cfg.Daemon.Port = DefaultAgentPort
	}

	// Ensure sub-structs exist if we want to apply defaults
	if cfg.Grub == nil {
		cfg.Grub = &GrubConfig{}
	}
	if cfg.Grub.WaitTimeSeconds == 0 {
		cfg.Grub.WaitTimeSeconds = DefaultGrubWaitSeconds
	}

	return &cfg, nil
}

func Save(cfg *Config, path string) error {
	maskWebHook := false
	out, err := cfg.ToYAML(maskWebHook)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, []byte(out), 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}
