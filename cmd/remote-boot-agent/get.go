package main

import (
	"context"
	"fmt"

	ha "github.com/jjack/remote-boot-agent/internal/homeassistant"

	"github.com/spf13/cobra"
)

func GetSelectedOS(cli *CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Output the currently selected OS from Home Assistant",
		RunE: func(cmd *cobra.Command, args []string) error {
			bl, err := ResolveBootloader(cli.Config)
			if err != nil {
				return err
			}

			if cli.Config.HomeAssistant.URL == "" {
				return fmt.Errorf("homeassistant url not configured")
			}

			haClient := ha.NewClient(cli.Config.HomeAssistant.URL, cli.Config.HomeAssistant.WebhookID)
			fmt.Printf("Fetching netboot configuration for hostname %s using bootloader %s...\n", cli.Config.Host.Hostname, bl.Name())

			response, err := haClient.View(context.Background(), bl.Name(), cli.Config.Host.MACAddress)
			if err != nil {
				return fmt.Errorf("failed to view configuration via HA API: %w", err)
			}

			fmt.Println(response)
			return nil
		},
	}
}
