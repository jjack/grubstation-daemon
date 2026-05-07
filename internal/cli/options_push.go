package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jjack/grub-os-reporter/internal/config"
	ha "github.com/jjack/grub-os-reporter/internal/homeassistant"

	"github.com/spf13/cobra"
)

var ErrMissingHAConfig = errors.New("homeassistant url and webhook_id must be configured")

func PushBootOptions(ctx context.Context, deps *CommandDeps) error {
	bootOptions, err := deps.Grub.GetBootOptions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get boot options: %w", err)
	}

	hostCfg := deps.Config.Host
	haCfg := deps.Config.HomeAssistant

	broadcastAddress := hostCfg.BroadcastAddress
	if broadcastAddress == config.DefaultBroadcastAddress {
		broadcastAddress = ""
	}
	broadcastPort := hostCfg.BroadcastPort
	if broadcastPort == config.DefaultBroadcastPort {
		broadcastPort = 0
	}

	payload := ha.PushPayload{
		MACAddress:       hostCfg.MACAddress,
		BroadcastAddress: broadcastAddress,
		BroadcastPort:    broadcastPort,
		Name:             hostCfg.Name,
		Address:          hostCfg.Address,
		BootOptions:      bootOptions,
	}

	if haCfg.URL == "" || haCfg.WebhookID == "" {
		return ErrMissingHAConfig
	}

	haClient := ha.NewClient(
		haCfg.URL,
		haCfg.WebhookID,
		nil,
	)

	slog.Debug("Pushing boot options to Home Assistant", "webhook_id", haCfg.WebhookID)
	slog.Debug("Payload", "payload", payload)

	if err := haClient.Push(ctx, payload); err != nil {
		return fmt.Errorf("failed to push boot options to HA webhook: %w", err)
	}

	slog.Debug("Successfully pushed boot options to Home Assistant")
	return nil
}

func NewPushCmd(deps *CommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "push",
		Short: "Push the list of available OSes to Home Assistant",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := PushBootOptions(cmd.Context(), deps); err != nil {
				return err
			}
			cmd.Println("Successfully pushed boot options to Home Assistant")
			return nil
		},
	}
}
