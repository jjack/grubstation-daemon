package reporter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"

	"github.com/jjack/grubstation-daemon/internal/config"
	"github.com/jjack/grubstation-daemon/internal/grub"
	ha "github.com/jjack/grubstation-daemon/internal/homeassistant"
	"github.com/jjack/grubstation-daemon/internal/version"
)

var ErrMissingHAConfig = errors.New("homeassistant url and webhook_id must be configured")

// Reporter handles pushing boot options to Home Assistant.
type Reporter struct {
	Config      *config.Config
	Grub        *grub.Grub
	ManagerName string
}

func New(cfg *config.Config, g *grub.Grub, managerName string) *Reporter {
	return &Reporter{
		Config:      cfg,
		Grub:        g,
		ManagerName: managerName,
	}
}

// RegisterDaemon performs the initial registration handshake with Home Assistant.
func (r *Reporter) RegisterDaemon(ctx context.Context, token string) error {
	hostCfg := r.Config.Host
	haCfg := r.Config.HomeAssistant
	daemonCfg := r.Config.Daemon

	payload := ha.RegistrationPayload{
		CommonPayload: ha.CommonPayload{
			Action:         ha.ActionRegisterAction,
			MACAddress:     hostCfg.MACAddress,
			Address:        hostCfg.Address,
			Version:        version.Version,
			OS:             runtime.GOOS,
			ServiceManager: r.ManagerName,
		},
		DaemonToken: token,
		DaemonPort:  daemonCfg.Port,
	}

	if haCfg.URL == "" || haCfg.WebhookID == "" {
		return ErrMissingHAConfig
	}

	haClient := ha.NewClient(
		haCfg.URL,
		haCfg.WebhookID,
		nil,
	)

	slog.Debug("Registering daemon with Home Assistant", "webhook_id", haCfg.WebhookID)
	if err := haClient.PostWebhook(ctx, payload); err != nil {
		return err
	}

	slog.Debug("Successfully registered daemon with Home Assistant")
	return nil
}

// PushBootOptions pushes the current GRUB boot options to Home Assistant.
func (r *Reporter) PushBootOptions(ctx context.Context) error {
	var bootOptions []string
	if r.Config.Daemon.ReportBootOptions {
		var err error
		bootOptions, err = r.Grub.GetBootOptions(ctx)
		if err != nil {
			return fmt.Errorf("failed to get boot options: %w", err)
		}
	}

	hostCfg := r.Config.Host
	haCfg := r.Config.HomeAssistant
	wolCfg := r.Config.WakeOnLan

	payload := ha.UpdatePayload{
		CommonPayload: ha.CommonPayload{
			Action:         ha.ActionUpdateAction,
			MACAddress:     hostCfg.MACAddress,
			Address:        hostCfg.Address,
			Version:        version.Version,
			OS:             runtime.GOOS,
			ServiceManager: r.ManagerName,
		},
		BootOptions:         bootOptions,
		WolBroadcastAddress: wolCfg.Address,
		WolBroadcastPort:    wolCfg.Port,
	}

	if haCfg.URL == "" || haCfg.WebhookID == "" {
		return ErrMissingHAConfig
	}

	haClient := ha.NewClient(
		haCfg.URL,
		haCfg.WebhookID,
		nil,
	)

	slog.Debug("Pushing boot options to Home Assistant", "webhook_id", haCfg.WebhookID, "payload", payload)
	if err := haClient.PostWebhook(ctx, payload); err != nil {
		return err
	}

	slog.Debug("Successfully pushed boot options to Home Assistant")
	return nil
}
