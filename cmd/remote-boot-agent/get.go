package main

import (
	"fmt"
	"log/slog"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
	ha "github.com/jjack/remote-boot-agent/internal/homeassistant"
	"github.com/spf13/cobra"
)

func NewGetRemoteBootOption(getBootloader func() (bootloader.Bootloader, error), getHAConfig func() config.HomeAssistantConfig, getHostConfig func() config.HostConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Output the currently selected boot option from Home Assistant",
		RunE: func(cmd *cobra.Command, args []string) error {
			bl, err := getBootloader()
			if err != nil {
				return err
			}

			haCfg := getHAConfig()
			if haCfg.URL == "" {
				return fmt.Errorf("homeassistant url not configured")
			}

			hostCfg := getHostConfig()
			haClient := ha.NewClient(haCfg.URL, haCfg.WebhookID)
			slog.Debug("Fetching netboot configuration for hostname %s using bootloader %s...\n", hostCfg.Hostname, bl.Name())

			response, err := haClient.View(cmd.Context(), bl.Name(), hostCfg.MACAddress)
			if err != nil {
				return fmt.Errorf("failed to view configuration via HA API: %w", err)
			}

			fmt.Println(response)
			return nil
		},
	}
}
