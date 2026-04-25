package main

import (
	"fmt"
	"log/slog"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
	ha "github.com/jjack/remote-boot-agent/internal/homeassistant"

	"github.com/spf13/cobra"
)

func NewPushBootOptions(getBootloader func() (bootloader.Bootloader, error), getBootloaderConfig func() config.BootloaderConfig, getHAConfig func() config.HomeAssistantConfig, getHostConfig func() config.HostConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "push",
		Short: "Push the list of available OSes to Home Assistant",
		RunE: func(cmd *cobra.Command, args []string) error {
			bl, err := getBootloader()
			if err != nil {
				return err
			}

			blCfg := getBootloaderConfig()
			bootOptions, err := bl.NewGetBootOptions(blCfg.ConfigPath)
			if err != nil {
				return fmt.Errorf("failed to get boot options: %w", err)
			}

			hostCfg := getHostConfig()
			payload := ha.PushPayload{
				MACAddress:  hostCfg.MACAddress,
				Bootloader:  bl.Name(),
				Hostname:    hostCfg.Hostname,
				BootOptions: bootOptions,
			}

			haCfg := getHAConfig()
			if haCfg.URL == "" || haCfg.WebhookID == "" {
				return fmt.Errorf("homeassistant url and webhook_id must be configured")
			}

			haClient := ha.NewClient(
				haCfg.URL,
				haCfg.WebhookID,
				nil,
			)

			slog.Info("Pushing boot options to Home Assistant", "webhook_id", haCfg.WebhookID)

			if err := haClient.Push(cmd.Context(), payload); err != nil {
				return fmt.Errorf("failed to push state to HA webhook: %w", err)
			}

			fmt.Println("Successfully pushed bootloader state.")
			return nil
		},
	}
}
