package reporter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jjack/grubstation/internal/config"
	"github.com/jjack/grubstation/internal/grub"
	ha "github.com/jjack/grubstation/internal/homeassistant"
)

var ErrMissingHAConfig = errors.New("homeassistant url and webhook_id must be configured")

// Reporter handles pushing boot options to Home Assistant.
type Reporter struct {
	Config      *config.Config
	Grub        *grub.Grub
	ManagerName string
	HAClient    *ha.Client
}

func New(cfg *config.Config, g *grub.Grub, managerName string) *Reporter {
	var haClient *ha.Client
	if cfg.HomeAssistant.URL != "" && cfg.HomeAssistant.WebhookID != "" {
		haClient = ha.NewClient(cfg.HomeAssistant.URL, cfg.HomeAssistant.WebhookID, nil)
	}

	return &Reporter{
		Config:      cfg,
		Grub:        g,
		ManagerName: managerName,
		HAClient:    haClient,
	}
}

// RegisterDaemon performs the initial registration handshake with Home Assistant.
func (r *Reporter) RegisterDaemon(ctx context.Context, token string) error {
	if r.HAClient == nil {
		return ErrMissingHAConfig
	}

	hostCfg := r.Config.Host
	daemonCfg := r.Config.Daemon

	payload := ha.RegistrationPayload{
		CommonPayload: ha.CommonPayload{
			Action:     ha.ActionRegisterAction,
			MACAddress: hostCfg.MACAddress,
			Address:    hostCfg.Address,
		},
		AgentToken: token,
		AgentPort:  daemonCfg.Port,
	}

	slog.Debug("Registering daemon with Home Assistant")
	if err := r.HAClient.PostWebhook(ctx, payload); err != nil {
		return err
	}

	slog.Debug("Successfully registered daemon with Home Assistant")
	return nil
}

// PushBootOptions pushes the current GRUB boot options to Home Assistant.
func (r *Reporter) PushBootOptions(ctx context.Context) error {
	if r.HAClient == nil {
		return ErrMissingHAConfig
	}

	var bootOptions []string
	if r.Config.Daemon.ReportBootOptions {
		var err error
		bootOptions, err = r.Grub.GetBootOptions(ctx)
		if err != nil {
			return fmt.Errorf("failed to get boot options: %w", err)
		}
	}

	hostCfg := r.Config.Host
	wolCfg := r.Config.WakeOnLan

	payload := ha.UpdatePayload{
		CommonPayload: ha.CommonPayload{
			Action:     ha.ActionUpdateAction,
			MACAddress: hostCfg.MACAddress,
			Address:    hostCfg.Address,
		},
		BootOptions: bootOptions,
	}

	if wolCfg != nil {
		payload.WolBroadcastAddress = wolCfg.Address
		payload.WolBroadcastPort = wolCfg.Port
	}

	slog.Debug("Pushing boot options to Home Assistant", "webhook_id", r.HAClient.WebhookID, "payload", payload)
	if err := r.HAClient.PostWebhook(ctx, payload); err != nil {
		return err
	}

	slog.Debug("Successfully pushed boot options to Home Assistant")
	return nil
}
