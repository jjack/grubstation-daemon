package config

import (
	"fmt"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config represents the loaded configuration from file or env vars
type Config struct {
	BootloaderName string `mapstructure:"bootloader"`
	InitSystemName string `mapstructure:"init_system"`
	HAWebhookURL   string `mapstructure:"ha_webhook_url"`
	// Device or hostname
}

// Load reads and parses configuration for the CLI application
func Load() (*Config, error) {
	pflag.String("config", "", "Explicit config file path (default is /etc/remote-boot-agent/config.yaml)")
	pflag.String("bootloader", "grub", "Name of the bootloader to use (default: grub)")
	pflag.String("init-system", "systemd", "Name of the init system to use (default: systemd)")
	pflag.String("homeassistant-url", "", "Home Assistant Base URL")
	pflag.String("homeassistant-token", "", "Home Assistant Long-Lived Access Token")
	pflag.Parse()

	viper.BindPFlag("bootloader", pflag.Lookup("bootloader"))
	viper.BindPFlag("init_system", pflag.Lookup("init-system"))
	viper.BindPFlag("home_assistant_url", pflag.Lookup("homeassistant-url"))
	viper.BindPFlag("home_assistant_auth_token", pflag.Lookup("homeassistant-token"))

	cfgFile, _ := pflag.CommandLine.GetString("config")
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Search for config in common locations
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("/etc/remote-boot-agent/")
		viper.AddConfigPath("$HOME/.config/remote-boot-agent/")
		viper.AddConfigPath(".")
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// File was found but contained errors
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// File not found; ignore and proceed with flags/defaults
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode into config struct: %w", err)
	}

	return &cfg, nil
}
