package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jjack/grubstation-daemon/internal/config"
	"github.com/jjack/grubstation-daemon/internal/daemon"
	"github.com/jjack/grubstation-daemon/internal/grub"
	"github.com/jjack/grubstation-daemon/internal/servicemanager"
)

type mockDaemonRunner struct {
	called *bool
	runErr error
}

func (m *mockDaemonRunner) Run(ctx context.Context) error {
	if m.called != nil {
		*m.called = true
	}
	return m.runErr
}

func TestNewDaemonCmd_RunEInvokesDaemon(t *testing.T) {
	oldNewDaemon := newDaemon
	defer func() { newDaemon = oldNewDaemon }()

	called := false
	newDaemon = func(cfg daemon.Config, pushHandler func(ctx context.Context, token string) error) daemonRunner {
		if cfg.Port != 1234 {
			t.Fatalf("expected listen port 1234, got %d", cfg.Port)
		}
		if !cfg.ReportBootOptions {
			t.Fatalf("expected ReportBootOptions to be true")
		}
		return &mockDaemonRunner{called: &called, runErr: errors.New("daemon run failed")}
	}

	cfg := &config.Config{
		Daemon: config.DaemonConfig{Port: 1234, ReportBootOptions: true},
	}
	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{ConfigPath: "/tmp/grub.cfg"}, Registry: servicemanager.NewRegistry()}
	cmd := NewDaemonCmd(deps)
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "daemon run failed") {
		t.Fatalf("expected daemon run failure, got %v", err)
	}
	if !called {
		t.Fatal("expected daemon Run to be invoked")
	}
}
