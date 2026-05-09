package cli

import (
	"bytes"
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/jjack/grub-os-reporter/internal/config"
)

func TestDefaultSystemResolver(t *testing.T) {
	// Ensure DefaultSystemResolver satisfies the SystemResolver interface
	var _ SystemResolver = (*DefaultSystemResolver)(nil)
	resolver := &DefaultSystemResolver{}

	// Short timeout so DiscoverHomeAssistant doesn't delay tests with mDNS
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// These are pass-throughs to the real system packages.
	// We just want to ensure they don't panic and are wired up correctly.
	_, _ = resolver.DiscoverHomeAssistant(ctx)
	_, _ = resolver.DetectSystemHostname()

	ifaces, _ := resolver.GetWOLInterfaces()
	if len(ifaces) > 0 {
		_, _ = resolver.GetIPv4Info(ifaces[0])
	} else {
		_, _ = resolver.GetIPv4Info(net.Interface{})
	}

	_ = resolver.GetFQDN("localhost")

	f, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	defer func() { _ = os.Remove(f.Name()) }()

	if err := resolver.SaveConfig(&config.Config{}, f.Name()); err != nil {
		t.Fatalf("expected no error saving config, got: %v", err)
	}
}

func TestNewCLI(t *testing.T) {
	cli := NewCLI()
	if cli == nil {
		t.Fatal("expected pointer to CLI, got nil")
	}
	if cli.RootCmd == nil {
		t.Fatal("expected RootCmd to be initialized")
	}
	if cli.RootCmd.Use != "grub-os-reporter" {
		t.Errorf("expected use 'grub-os-reporter', got %s", cli.RootCmd.Use)
	}
}

func TestCLI_Execute(t *testing.T) {
	cli := NewCLI()

	cli.RootCmd.SetArgs([]string{"help"})

	var b bytes.Buffer
	cli.RootCmd.SetOut(&b)

	err := cli.Execute()
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
}

func TestCLI_PersistentPreRun_ConfigParseFail(t *testing.T) {
	f, err := os.CreateTemp("", "bad-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	_, _ = f.Write([]byte("invalid\n yaml\n  content"))
	_ = f.Close()

	cli := NewCLI()
	cli.RootCmd.SetArgs([]string{"options", "list", "--config", f.Name()})
	err = cli.Execute()
	if err == nil {
		t.Fatal("expected error on malformed config file")
	}
}

func TestCLI_PersistentPreRun_ConfigValidateFail(t *testing.T) {
	f, err := os.CreateTemp("", "empty-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	_, _ = f.Write([]byte("{}")) // Empty config will parse successfully, but fail domain validation
	_ = f.Close()

	cli := NewCLI()
	cli.RootCmd.SetArgs([]string{"options", "list", "--config", f.Name()})
	err = cli.Execute()
	if err == nil {
		t.Fatal("expected error on invalid config file")
	}
}

func TestCLI_PersistentPreRun_Setup(t *testing.T) {
	cli := NewCLI()

	cmd, _, err := cli.RootCmd.Find([]string{"setup"})
	if err != nil {
		t.Fatal(err)
	}

	if cmd.PersistentPreRunE == nil {
		t.Fatal("expected setup command to override PersistentPreRunE")
	}

	err = cmd.PersistentPreRunE(cmd, []string{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}
