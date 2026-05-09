package reporter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jjack/grubstation-cli/internal/config"
	"github.com/jjack/grubstation-cli/internal/grub"
	ha "github.com/jjack/grubstation-cli/internal/homeassistant"
	"github.com/jjack/grubstation-cli/internal/version"
)

var ErrMissingHAConfig = errors.New("homeassistant url and webhook_id must be configured")

// Reporter handles pushing boot options to Home Assistant.
type Reporter struct {
	Config      *config.Config
	Grub        *grub.Grub
	ServiceName string
}

func New(cfg *config.Config, g *grub.Grub, serviceName string) *Reporter {
	return &Reporter{
		Config:      cfg,
		Grub:        g,
		ServiceName: serviceName,
	}
}

// PushBootOptions pushes the current GRUB boot options to Home Assistant.
func (r *Reporter) PushBootOptions(ctx context.Context, token string) error {
	var bootOptions []string
	var err error
	if r.Config.Daemon.ReportBootOptions {
		bootOptions, err = r.Grub.GetBootOptions(ctx)
		if err != nil {
			return fmt.Errorf("failed to get boot options: %w", err)
		}
	}

	hostCfg := r.Config.Host
	wolCfg := r.Config.WakeOnLan
	haCfg := r.Config.HomeAssistant
	daemonCfg := r.Config.Daemon

	payload := ha.PushPayload{
		MACAddress:   hostCfg.MACAddress,
		WolAddress:   wolCfg.Address,
		WolPort:      wolCfg.Port,
		Name:         hostCfg.Name,
		Address:      hostCfg.Address,
		BootOptions:  bootOptions,
		APIToken:     token,
		AgentVersion: version.Version,
		AgentPort:    daemonCfg.ListenPort,
		Service:      r.ServiceName,
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
