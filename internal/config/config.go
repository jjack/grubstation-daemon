package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Host          HostConfig          `mapstructure:"host"`
	Bootloader    BootloaderConfig    `mapstructure:"bootloader"`
	HomeAssistant HomeAssistantConfig `mapstructure:"homeassistant"`
}

type BootloaderConfig struct {
	Name       string `mapstructure:"name"`
	ConfigPath string `mapstructure:"config_path"`
}

type HostConfig struct {
	MACAddress string `mapstructure:"mac_address"`
	Hostname   string `mapstructure:"hostname"`
}

type HomeAssistantConfig struct {
	URL       string `mapstructure:"url"`
	WebhookID string `mapstructure:"webhook_id"`
}

func LoadConfig(cfgFile string) (*Config, error) {
	v := viper.New()
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.AddConfigPath("/etc/ha-remote-boot-agent/")
		v.AddConfigPath(os.ExpandEnv("$HOME/.config/ha-remote-boot-agent/"))
		v.AddConfigPath(".")
		v.SetConfigName("config")
		v.SetConfigType("yaml")
	}

	v.AutomaticEnv()
	v.SetEnvPrefix("RBA")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	v.SetDefault("homeassistant.url", "http://homeassistant.local:8123")
	v.SetDefault("homeassistant.webhook_id", "remote_boot_manager_ingest")

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
