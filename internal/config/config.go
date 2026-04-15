package config

import (
	"fmt"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config represents the loaded configuration from file or env vars
type Config struct {
	HomeAssistant HAConfig   `mapstructure:"homeassistant"`
	Host          HostConfig `mapstructure:"host"`
}

type HostConfig struct {
	Bootloader string `mapstructure:"bootloader"`
	InitSystem string `mapstructure:"initsystem"`
	MACAddress string `mapstructure:"mac_address"`
	Hostname   string `mapstructure:"hostname"`
}

type HAConfig struct {
	BaseURL string `mapstructure:"url"`
	Token   string `mapstructure:"token"`
}

func InitFlags(flags *pflag.FlagSet) {
	flags.String("config", "", "Explicit config file path (default is /etc/remote-boot-agent/config.yaml)")
	flags.String("bootloader", "", "Name of the bootloader to use (optional, will be auto-detected if not provided)")
	flags.String("init-system", "", "Name of the init system to use (optional, will be auto-detected if not provided)")
	flags.String("mac-address", "", "MAC address of the device (optional, will be auto-detected if not provided)")
	flags.String("hostname", "", "Hostname of the device (optional, will be auto-detected if not provided)")

	flags.String("homeassistant-url", "", "Home Assistant Base URL")
	flags.String("homeassistant-token", "", "Home Assistant Long-Lived Access Token")
}

// Load reads and parses configuration for the CLI application
func Load(flags *pflag.FlagSet) (*Config, error) {
	v := viper.New()

	v.BindPFlag("host.bootloader", flags.Lookup("bootloader"))
	v.BindPFlag("host.initsystem", flags.Lookup("init-system"))
	v.BindPFlag("host.mac_address", flags.Lookup("mac-address"))
	v.BindPFlag("host.hostname", flags.Lookup("hostname"))
	v.BindPFlag("homeassistant.url", flags.Lookup("homeassistant-url"))
	v.BindPFlag("homeassistant.token", flags.Lookup("homeassistant-token"))

	cfgFile, _ := flags.GetString("config")
	if cfgFile != "" {
		// Use config file from the flag
		v.SetConfigFile(cfgFile)
	} else {
		// Search for config in common locations
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("/etc/remote-boot-agent/")
		v.AddConfigPath("$HOME/.config/remote-boot-agent/")
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
	if cfg.Host.Hostname == "" || cfg.Host.MACAddress == "" {
		host, mac := discoverNetworkInfo()
		if cfg.Host.Hostname == "" {
			cfg.Host.Hostname = host
		}
		if cfg.Host.MACAddress == "" {
			cfg.Host.MACAddress = mac
		}
	}

	return &cfg, nil
}
