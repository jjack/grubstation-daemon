//go:build linux

package daemon

import (
	"context"
	"log/slog"
	"time"
)

func onShutdownHook(ctx context.Context, d *Daemon) {
	if d.Config.ReportBootOptions && d.UpdateHandler != nil {
		slog.Info("Performing final GRUB report push")
		pushCtx, pushCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer pushCancel()
		if err := d.UpdateHandler(pushCtx); err != nil {
			slog.Error("Final push failed", "error", err)
		}
	}
}
