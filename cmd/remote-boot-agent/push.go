package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
	ha "github.com/jjack/remote-boot-agent/internal/homeassistant"

	"github.com/spf13/cobra"
)

func NewPushBootOptions(getBootloader func() (bootloader.Bootloader, error), getConfig func() *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "push",
		Short: "Push the list of available OSes to Home Assistant",
		RunE: func(cmd *cobra.Command, args []string) error {
			bl, err := getBootloader()
			if err != nil {
				return err
			}

			cfg := getConfig()
			bootOptions, err := bl.NewGetBootOptions(cfg.Bootloader.ConfigPath)
			if err != nil {
				return fmt.Errorf("failed to get boot options: %w", err)
			}

			payload := ha.PushPayload{
				MACAddress:  cfg.Host.MACAddress,
				Bootloader:  bl.Name(),
				Hostname:    cfg.Host.Hostname,
				BootOptions: bootOptions,
			}

			if cfg.HomeAssistant.URL == "" || cfg.HomeAssistant.WebhookID == "" {
				return fmt.Errorf("homeassistant url and webhook_id must be configured")
			}

			haClient := ha.NewClient(
				cfg.HomeAssistant.URL,
				cfg.HomeAssistant.WebhookID,
			)

			slog.Info("Pushing boot options to Home Assistant", "webhook_id", cfg.HomeAssistant.WebhookID)

			if err := haClient.Push(context.Background(), payload); err != nil {
				return fmt.Errorf("failed to push state to HA webhook: %w", err)
			}

			fmt.Println("Successfully pushed bootloader state.")
			return nil
		},
	}
}
