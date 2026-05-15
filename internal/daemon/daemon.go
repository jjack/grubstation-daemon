package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"time"
)

var (
	execCommand = exec.Command
	randRead    = rand.Read
)

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := randRead(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Config holds the daemon configuration.
type Config struct {
	Port              int
	ReportBootOptions bool
	APIKey            string
	RetryInterval     time.Duration
	ShutdownDelay     time.Duration
}

// Metadata holds system information.
type Metadata struct {
	OS             string `json:"os"`
	Version        string `json:"version"`
	ServiceManager string `json:"service_manager"`
}

// Daemon represents the background service.
type Daemon struct {
	Config          Config
	Metadata        Metadata
	RegisterHandler func(ctx context.Context, token string) error
	UpdateHandler   func(ctx context.Context) error
}

func New(cfg Config, meta Metadata, regHandler func(ctx context.Context, token string) error, updateHandler func(ctx context.Context) error) *Daemon {
	return &Daemon{
		Config:          cfg,
		Metadata:        meta,
		RegisterHandler: regHandler,
		UpdateHandler:   updateHandler,
	}
}

// run contains the core daemon logic.
func (d *Daemon) run(ctx context.Context) error {
	token := d.Config.APIKey
	if token == "" {
		var err error
		token, err = generateToken()
		if err != nil {
			return fmt.Errorf("failed to generate dynamic token: %w", err)
		}
		slog.Info("Using dynamically generated TOFU token")
	} else {
		slog.Info("Using configured API key")
	}

	go d.listenUnixSocket(ctx, token)

	// 1. Initial Handshake (Register + First Update) with Retry logic
	if d.RegisterHandler != nil {
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
					if err := d.RegisterHandler(ctx, token); err != nil {
						slog.Warn("Initial registration failed, retrying...", "error", err, "retry_in", backoff)
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
					slog.Info("Initial registration successful")

					// Immediately send first update after successful registration
					if d.UpdateHandler != nil {
						if err := d.UpdateHandler(ctx); err != nil {
							slog.Error("Initial update failed", "error", err)
						} else {
							slog.Info("Initial update successful")
						}
					}
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
			if r.Method == http.MethodGet && r.URL.Path == "/status" {
				status := struct {
					Status string `json:"status"`
					Metadata
				}{
					Status:   "ok",
					Metadata: d.Metadata,
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(status)
				return
			}

			if r.URL.Path == "/shutdown" {
				if r.Method != http.MethodPost {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusMethodNotAllowed)
					_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": "Method not allowed"})
					return
				}

				auth := r.Header.Get("Authorization")
				if auth != "Bearer "+token {
					slog.Warn("Unauthorized shutdown request", "remote_addr", r.RemoteAddr)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": "Forbidden"})
					return
				}

				slog.Info("Shutdown requested via HTTP")

				// Perform pre-shutdown push (synchronous)
				if d.Config.ReportBootOptions && d.UpdateHandler != nil {
					slog.Info("Performing pre-shutdown GRUB report push")
					if err := d.UpdateHandler(ctx); err != nil {
						slog.Error("Pre-shutdown push failed", "error", err)
					}
				}

				if err := d.performOSShutdown(); err != nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": err.Error()})
					return
				}

				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
				return
			}

			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

	onShutdownHook(ctx, d)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func (d *Daemon) performOSShutdown() error {
	cmd := getShutdownCommand()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute shutdown command: %w", err)
	}
	return nil
}
