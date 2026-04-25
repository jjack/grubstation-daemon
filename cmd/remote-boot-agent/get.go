package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jjack/remote-boot-agent/internal/config"
	ha "github.com/jjack/remote-boot-agent/internal/homeassistant"
	"github.com/spf13/cobra"
)

func NewGetRemoteBootOption(getConfig func() *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Output the currently selected boot option from Home Assistant",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := getConfig()
			bl, err := ResolveBootloader(cfg.Bootloader.Name)
			if err != nil {
				return err
			}

			if cfg.HomeAssistant.URL == "" {
				return fmt.Errorf("homeassistant url not configured")
			}

			haClient := ha.NewClient(cfg.HomeAssistant.URL, cfg.HomeAssistant.WebhookID)
			slog.Debug("Fetching netboot configuration for hostname %s using bootloader %s...\n", cfg.Host.Hostname, bl.Name())

			response, err := haClient.View(context.Background(), bl.Name(), cfg.Host.MACAddress)
			if err != nil {
				return fmt.Errorf("failed to view configuration via HA API: %w", err)
			}

			fmt.Println(response)
			return nil
		},
	}
}
