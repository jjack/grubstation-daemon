package config

// Config represents the loaded configuration from file or env vars
type Config struct {
	BootloaderName string
	InitSystemName string
	HAWebhookURL   string
	// Device or hostname
}

// Load reads and parses configuration for the CLI application
func Load() (*Config, error) {
	// TODO: use Viper, Kingpin, Cobra or standard library flags
	return &Config{}, nil
}
