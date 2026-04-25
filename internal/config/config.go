package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
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

func RegisterFlags(flags *pflag.FlagSet) {
	flags.String("mac", "", "MAC Address override")
	flags.String("hostname", "", "Hostname override")
	flags.String("bootloader", "", "Bootloader type override (e.g., grub)")
	flags.String("bootloader-path", "", "Bootloader config path override")
	flags.String("hass-url", "", "Home Assistant URL override")
	flags.String("hass-webhook", "", "Home Assistant Webhook ID override")
}

func Load(cfgFile string, flags *pflag.FlagSet) (*Config, error) {
	v := viper.New()
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.AddConfigPath("/etc/remote-boot-agent/")
		v.AddConfigPath(os.ExpandEnv("$HOME/.config/remote-boot-agent/"))
		v.AddConfigPath(".")
		v.SetConfigName("config")
		v.SetConfigType("yaml")
	}

	v.AutomaticEnv()
	v.SetEnvPrefix("RBA")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if flags != nil {
		_ = v.BindPFlag("host.mac_address", flags.Lookup("mac"))
		_ = v.BindPFlag("host.hostname", flags.Lookup("hostname"))
		_ = v.BindPFlag("bootloader.name", flags.Lookup("bootloader"))
		_ = v.BindPFlag("bootloader.config_path", flags.Lookup("bootloader-path"))
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
	v.Set("host.mac_address", cfg.Host.MACAddress)
	v.Set("host.hostname", cfg.Host.Hostname)
	v.Set("homeassistant.url", cfg.HomeAssistant.URL)
	v.Set("homeassistant.webhook_id", cfg.HomeAssistant.WebhookID)

	if err := v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}
