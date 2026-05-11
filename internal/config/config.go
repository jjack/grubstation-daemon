package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const (
	DefaultWolAddress = "255.255.255.255"
	DefaultWolPort    = 9
)

func DefaultConfigPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("AppData"), "grubstation", "config.yaml")
	}
	return "/etc/grubstation/config.yaml"
}

const (
	FlagGrubConfig  = "grub-config"
	FlagMac         = "host-mac"
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
	Host          HostConfig          `yaml:"host"`
	WakeOnLan     WakeOnLanConfig     `yaml:"wake_on_lan"`
	HomeAssistant HomeAssistantConfig `yaml:"homeassistant"`
	Grub          GrubConfig          `yaml:"grub"`
	Daemon        DaemonConfig        `yaml:"daemon"`
}

type DaemonConfig struct {
	ListenPort        int    `yaml:"listen_port"`
	APIKey            string `yaml:"api_key,omitempty"`
	ReportBootOptions bool   `yaml:"report_boot_options"`
}

type GrubConfig struct {
	ConfigPath      string `yaml:"config_path,omitempty"`
	WaitTimeSeconds int    `yaml:"wait_time_seconds,omitempty"`
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

	// Apply suppression for default values to match existing logic
	if displayCfg.WakeOnLan.Address == DefaultWolAddress {
		displayCfg.WakeOnLan.Address = ""
	}
	if displayCfg.WakeOnLan.Port == DefaultWolPort {
		displayCfg.WakeOnLan.Port = 0
	}
	if displayCfg.Grub.WaitTimeSeconds == 2 {
		displayCfg.Grub.WaitTimeSeconds = 0
	}

	if maskWebhook && len(displayCfg.HomeAssistant.WebhookID) > 4 {
		displayCfg.HomeAssistant.WebhookID = displayCfg.HomeAssistant.WebhookID[:4] + "..."
	}

	out, err := yaml.Marshal(displayCfg)
	if err != nil {
		return "", err
	}
	
	// Final cleanup: if wake_on_lan or grub are empty, remove them from output
	// This is a bit of a hack but avoids pointers which complicate the rest of the app.
	lines := strings.Split(string(out), "\n")
	var finalLines []string
	skipNextIfEmpty := map[string]bool{"wake_on_lan: {}": true, "grub: {}": true}
	for _, line := range lines {
		if !skipNextIfEmpty[line] {
			finalLines = append(finalLines, line)
		}
	}

	return strings.Join(finalLines, "\n"), nil
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
	// Use "yaml" tags for unmarshaling to avoid redundancy
	if err := v.Unmarshal(&cfg, func(dc *mapstructure.DecoderConfig) {
		dc.TagName = "yaml"
	}); err != nil {
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
	out, err := cfg.ToYAML(false)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, []byte(out), 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}
