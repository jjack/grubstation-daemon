package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jjack/grubstation/internal/config"
	"github.com/jjack/grubstation/internal/daemon"
	"github.com/jjack/grubstation/internal/grub"
	"github.com/jjack/grubstation/internal/servicemanager"
)

type mockServeRunner struct {
	called *bool
	runErr error
}

func (m *mockServeRunner) Run(ctx context.Context) error {
	if m.called != nil {
		*m.called = true
	}
	return m.runErr
}

func TestNewServeCmd_RunEInvokesDaemon(t *testing.T) {
	oldNewServe := newServe
	defer func() { newServe = oldNewServe }()

	called := false
	newServe = func(cfg daemon.Config, meta daemon.Metadata, regHandler func(ctx context.Context, token string) error, updateHandler func(ctx context.Context) error) serveRunner {
		if cfg.Port != 1234 {
			t.Fatalf("expected listen port 1234, got %d", cfg.Port)
		}
		if !cfg.ReportBootOptions {
			t.Fatalf("expected ReportBootOptions to be true")
		}
		return &mockServeRunner{called: &called, runErr: errors.New("serve run failed")}
	}

	cfg := &config.Config{
		Daemon: config.DaemonConfig{Port: 1234, ReportBootOptions: true},
	}
	deps := &CommandDeps{Config: cfg, Grub: &grub.Grub{ConfigPath: "/tmp/grub.cfg"}, Registry: servicemanager.NewRegistry()}
	cmd := NewServeCmd(deps)
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "serve run failed") {
		t.Fatalf("expected serve run failure, got %v", err)
	}
	if !called {
		t.Fatal("expected serve Run to be invoked")
	}
}
