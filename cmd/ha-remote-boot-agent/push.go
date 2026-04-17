package main

import (
	"context"
	"fmt"

	ha "github.com/jjack/ha-remote-boot-agent/internal/homeassistant"

	"github.com/spf13/cobra"
)

func PushBootOptions(cli *CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "push",
		Short: "Push the list of available OSes to Home Assistant",
		RunE: func(cmd *cobra.Command, args []string) error {
			bl, err := ResolveBootloader(cli.Config)
			if err != nil {
				return err
			}

			bootOptions, err := bl.GetBootOptions(cli.Config.Bootloader.ConfigPath)
			if err != nil {
				return fmt.Errorf("failed to get boot options: %w", err)
			}

			payload := ha.PushPayload{
				MACAddress:  cli.Config.Host.MACAddress,
				Bootloader:  bl.Name(),
				Hostname:    cli.Config.Host.Hostname,
				BootOptions: bootOptions,
			}

			if cli.Config.HomeAssistant.URL == "" || cli.Config.HomeAssistant.WebhookID == "" {
				return fmt.Errorf("homeassistant url and webhook_id must be configured")
			}

			haClient := ha.NewClient(
				cli.Config.HomeAssistant.URL,
				cli.Config.HomeAssistant.WebhookID,
			)

			fmt.Printf("Pushing state to Home Assistant (Webhook ID: %s)\n", cli.Config.HomeAssistant.WebhookID)

			if err := haClient.Push(context.Background(), payload); err != nil {
				return fmt.Errorf("failed to push state to HA webhook: %w", err)
			}

			fmt.Println("Successfully pushed bootloader state.")
			return nil
		},
	}
}
