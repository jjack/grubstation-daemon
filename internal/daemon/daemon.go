package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"
)

var (
	execCommand = exec.Command
	osExit      = os.Exit
)

// Config holds the daemon configuration.
type Config struct {
	Port              int
	ReportBootOptions bool
	APIKey            string
	RetryInterval     time.Duration
	ShutdownDelay     time.Duration
}

// Daemon represents the background service.
type Daemon struct {
	Config      Config
	PushHandler func(ctx context.Context, token string) error
}

func New(cfg Config, pushHandler func(ctx context.Context, token string) error) *Daemon {
	return &Daemon{
		Config:      cfg,
		PushHandler: pushHandler,
	}
}

// run contains the core daemon logic.
func (d *Daemon) run(ctx context.Context) error {
	token := d.Config.APIKey
	if token == "" {
		return fmt.Errorf("API key must be configured")
	}
	slog.Info("Using configured API key")

	go d.listenUnixSocket(ctx, token)

	// 1. Initial Handshake with Retry logic
	if d.PushHandler != nil {
		go func() {
			backoff := d.Config.RetryInterval
			if backoff == 0 {
				backoff = 5 * time.Second
			}
			maxBackoff := 5 * time.Minute
			for {
				select {
				case <-ctx.Done():
					return
				default:
					if err := d.PushHandler(ctx, token); err != nil {
						slog.Error("Initial handshake failed, retrying...", "error", err, "retry_in", backoff)
						select {
						case <-ctx.Done():
							return
						case <-time.After(backoff):
						}
						backoff *= 2
						if backoff > maxBackoff {
							backoff = maxBackoff
						}
						continue
					}
					slog.Info("Initial handshake successful")
					return
				}
			}
		}()
	}

	// 2. Start HTTP Server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", d.Config.Port),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == "/healthcheck" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok\n"))
				return
			}

			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			auth := r.Header.Get("Authorization")
			if auth != "Bearer "+token {
				slog.Warn("Unauthorized shutdown request", "remote_addr", r.RemoteAddr)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			if r.URL.Path == "/shutdown" {
				slog.Info("Shutdown requested via HTTP")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Shutting down...\n"))

				// Execute final push and shutdown in a goroutine
				go func() {
					// Final push before OS shutdown if we are via HTTP
					if d.Config.ReportBootOptions && d.PushHandler != nil {
						slog.Info("Performing pre-shutdown GRUB report push")
						if err := d.PushHandler(ctx, token); err != nil {
							slog.Error("Pre-shutdown push failed", "error", err)
						}
					}
					d.performOSShutdown()
				}()
				return
			}

			http.NotFound(w, r)
		}),
	}

	go func() {
		slog.Info("Starting HTTP listener", "port", d.Config.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server failed", "error", err)
		}
	}()

	slog.Info("Daemon is running and waiting for termination")

	// 3. Finalization logic when context is cancelled
	<-ctx.Done()
	slog.Info("Shutting down daemon...")

	// Final push if GRUB is enabled (for manual SIGTERM/systemd stop)
	if d.Config.ReportBootOptions && runtime.GOOS == "linux" && d.PushHandler != nil {
		slog.Info("Performing final GRUB report push")
		pushCtx, pushCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer pushCancel()
		if err := d.PushHandler(pushCtx, token); err != nil {
			slog.Error("Final push failed", "error", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func (d *Daemon) performOSShutdown() {
	// Wait a bit to ensure the HTTP response is sent
	delay := 1 * time.Second
	if d.Config.ShutdownDelay != 0 {
		delay = d.Config.ShutdownDelay
	}
	time.Sleep(delay)

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = execCommand("shutdown", "/s", "/t", "0")
	} else {
		cmd = execCommand("poweroff")
	}

	if err := cmd.Run(); err != nil {
		slog.Error("Failed to execute shutdown command", "error", err)
		osExit(1)
	}
}
