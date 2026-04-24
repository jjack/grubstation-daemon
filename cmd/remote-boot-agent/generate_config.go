package main

import (
	"fmt"

	"github.com/charmbracelet/huh/spinner"
	"github.com/jjack/remote-boot-agent/internal/config"
	"github.com/jjack/remote-boot-agent/internal/homeassistant"
	"github.com/jjack/remote-boot-agent/internal/system"
	"github.com/spf13/cobra"
)

// NewGenerateConfigCmd walks the user through generating a config interactively
func NewGenerateConfigCmd(cli *CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "generate-config",
		Short: "Interactively generate a config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			var hassURL string

			_ = spinner.New().
				Title("Scanning network for Home Assistant...").
				Action(func() {
					url, err := homeassistant.Discover()
					if err == nil && url != "" {
						hassURL = url
					}
				}).
				Run()

			hostname, err := system.DetectHostname()
			if err != nil {
				return err
			}

			interfaces, err := system.GetInterfaceOptions()
			if err != nil {
				return err
			}

			cfg, err := GenerateConfigForm(hostname, hassURL, interfaces)
			if err != nil {
				return err
			}

			fmt.Println("\nGenerated config (keys may be in a different order than shown here):")
			fmt.Printf("---\n")
			fmt.Printf("host:\n  hostname: %s\n  mac: %s\n", cfg.Host.Hostname, cfg.Host.MACAddress)
			fmt.Printf("homeassistant:\n  url: %s\n  webhook_id: %s\n", cfg.HomeAssistant.URL, cfg.HomeAssistant.WebhookID)

			return config.Save(cfg, "./config.yaml")
		},
	}
}
