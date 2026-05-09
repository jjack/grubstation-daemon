//go:build !windows

package daemon

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
)

var SocketPath = "/var/run/grub-os-reporter.sock"

// Run starts the daemon and waits for termination signals.
func (d *Daemon) Run(ctx context.Context) error {
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

func (d *Daemon) listenUnixSocket(ctx context.Context, token string) {
	path := SocketPath
	_ = os.Remove(path)
	l, err := net.Listen("unix", path)
	if err != nil {
		slog.Debug("Failed to create unix socket", "error", err)
		return
	}
	defer l.Close()
	defer os.Remove(path)

	go func() {
		<-ctx.Done()
		_ = l.Close()
	}()

	for {
		conn, err := l.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				slog.Debug("Unix socket accept error", "error", err)
				continue
			}
		}
		go d.handleUnixConnection(ctx, conn, token)
	}
}

func (d *Daemon) handleUnixConnection(ctx context.Context, conn net.Conn, token string) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	if scanner.Scan() {
		if cmd := scanner.Text(); cmd == "push" {
			if d.PushHandler != nil {
				slog.Info("Push requested via local Unix socket")
				if err := d.PushHandler(ctx, token); err != nil {
					slog.Error("Socket requested push failed", "error", err)
					_, _ = conn.Write([]byte(fmt.Sprintf("ERROR: %v\n", err)))
				} else {
					_, _ = conn.Write([]byte("OK\n"))
				}
			} else {
				_, _ = conn.Write([]byte("ERROR: PushHandler not configured\n"))
			}
		}
	}
}

func RequestPushViaSocket(ctx context.Context) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", SocketPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err = conn.Write([]byte("push\n")); err != nil {
		return err
	}

	scanner := bufio.NewScanner(conn)
	if scanner.Scan() {
		if resp := scanner.Text(); resp == "OK" {
			return nil
		} else {
			return fmt.Errorf("daemon returned error: %s", resp)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return fmt.Errorf("no response from daemon")
}
