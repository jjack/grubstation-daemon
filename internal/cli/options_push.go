package cli

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	ha "github.com/jjack/remote-boot-agent/internal/homeassistant"

	"github.com/spf13/cobra"
)

var ErrMissingHAConfig = errors.New("homeassistant url and webhook_id must be configured")

func NewPushCmd(deps *CommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "push",
		Short: "Push the list of available OSes to Home Assistant",
		RunE: func(cmd *cobra.Command, args []string) error {
			bl, err := deps.Bootloader(cmd.Context())
			if err != nil {
				return err
			}

			blCfg := deps.Config.Bootloader
			bootOptions, err := bl.GetBootOptions(cmd.Context(), bootloader.Config{
				ConfigPath: blCfg.ConfigPath,
			})
			if err != nil {
				return fmt.Errorf("failed to get boot options: %w", err)
			}

			hostCfg := deps.Config.Host
			payload := ha.PushPayload{
				MACAddress:       hostCfg.MACAddress,
				BroadcastAddress: hostCfg.BroadcastAddress,
				BroadcastPort:    hostCfg.BroadcastPort,
				Bootloader:       bl.Name(),
				Hostname:         hostCfg.Hostname,
				BootOptions:      bootOptions,
			}

			haCfg := deps.Config.HomeAssistant
			if haCfg.URL == "" || haCfg.WebhookID == "" {
				return ErrMissingHAConfig
			}

			haClient := ha.NewClient(
				haCfg.URL,
				haCfg.WebhookID,
				nil,
			)

			slog.Info("Pushing boot options to Home Assistant", "webhook_id", haCfg.WebhookID)
			slog.Debug("Payload", "payload", payload)

			if err := haClient.Push(cmd.Context(), payload); err != nil {
				return fmt.Errorf("failed to push state to HA webhook: %w", err)
			}

			slog.Info("Successfully pushed bootloader state to Home Assistant")
			return nil
		},
	}
}
