package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jjack/grub-os-reporter/internal/config"
	"github.com/jjack/grub-os-reporter/internal/daemon"
	"github.com/jjack/grub-os-reporter/internal/grub"
	"github.com/jjack/grub-os-reporter/internal/service"
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
		if cfg.ListenPort != 1234 {
			t.Fatalf("expected listen port 1234, got %d", cfg.ListenPort)
		}
		if !cfg.ReportBootOptions {
			t.Fatalf("expected ReportBootOptions to be true")
		}
		return &mockDaemonRunner{called: &called, runErr: errors.New("daemon run failed")}
	}

	cfg := &config.Config{
		Daemon: config.DaemonConfig{ListenPort: 1234, ReportBootOptions: true},
	}
	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{ConfigPath: "/tmp/grub.cfg"}, ServiceRegistry: service.NewRegistry()}
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
