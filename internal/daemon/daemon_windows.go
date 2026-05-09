//go:build windows

package daemon

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sys/windows/svc"
)

const windowsServiceName = "GrubStation"

// Run starts the daemon, optionally as a Windows service.
func (d *Daemon) Run(ctx context.Context) error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return err
	}

	if isService {
		return svc.Run(windowsServiceName, &serviceHandler{d: d, ctx: ctx})
	}

	// Not a service, handle signals manually
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		slog.Info("Received signal, stopping daemon", "signal", sig)
		cancel()
	}()

	return d.run(ctx)
}

type serviceHandler struct {
	d   *Daemon
	ctx context.Context
}

func (m *serviceHandler) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(m.ctx)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- m.d.run(ctx)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

loop:
	for {
		select {
		case err := <-errChan:
			if err != nil {
				slog.Error("Daemon failed", "error", err)
			}
			break loop
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				cancel()
				break loop
			default:
				slog.Error("Unexpected control request", "cmd", c.Cmd)
			}
		}
	}

	changes <- svc.Status{State: svc.StopPending}
	return
}

func (d *Daemon) listenUnixSocket(ctx context.Context, token string) {
	// No-op on Windows
}

func RequestPushViaSocket(ctx context.Context) error {
	return errors.New("unix sockets not supported on windows")
}
