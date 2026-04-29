package cli

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
	"github.com/jjack/remote-boot-agent/internal/initsystem"
)

func TestNewCLI(t *testing.T) {
	cli := NewCLI()
	if cli == nil {
		t.Fatal("expected pointer to CLI, got nil")
	}
	if cli.RootCmd == nil {
		t.Fatal("expected RootCmd to be initialized")
	}
	if cli.RootCmd.Use != "remote-boot-agent" {
		t.Errorf("expected use 'remote-boot-agent', got %s", cli.RootCmd.Use)
	}
}

func TestResolveBootloader(t *testing.T) {
	cfg := &config.Config{
		Bootloader: config.BootloaderConfig{
			Name: "example",
		},
	}

	registry := bootloader.NewRegistry()
	registry.Register("example", bootloader.NewExample)

	bl, err := ResolveBootloader(context.Background(), cfg.Bootloader.Name, registry)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if bl.Name() != "example" {
		t.Errorf("expected 'example', got %s", bl.Name())
	}

	// Invalid bootloader
	cfgInvalid := &config.Config{
		Bootloader: config.BootloaderConfig{
			Name: "invalid-bootloader",
		},
	}
	_, errInvalid := ResolveBootloader(context.Background(), cfgInvalid.Bootloader.Name, registry)
	if errInvalid == nil {
		t.Fatal("expected error for invalid bootloader")
	}

	// Empty bootloader string triggers Detect
	cfgEmpty := &config.Config{
		Bootloader: config.BootloaderConfig{
			Name: "",
		},
	}
	// example always returns true for IsActive so Detect will find it
	blDetect, errDetect := ResolveBootloader(context.Background(), cfgEmpty.Bootloader.Name, registry)
	if errDetect != nil {
		t.Fatalf("expected no error detecting, got %v", errDetect)
	}
	if blDetect == nil {
		t.Fatal("expected detected bootloader to not be nil")
	}
}

func TestResolveInitSystem(t *testing.T) {
	cfg := &config.Config{
		InitSystem: config.InitSystemConfig{
			Name: "mock",
		},
	}

	registry := initsystem.NewRegistry()
	// We'll borrow the systemd struct since it's the only one we have
	registry.Register("mock", initsystem.NewSystemd)

	sys, err := ResolveInitSystem(context.Background(), cfg.InitSystem.Name, registry)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if sys.Name() != "systemd" {
		t.Errorf("expected 'systemd', got %s", sys.Name())
	}

	// Invalid init system
	cfgInvalid := &config.Config{
		InitSystem: config.InitSystemConfig{
			Name: "invalid-initsys",
		},
	}
	_, errInvalid := ResolveInitSystem(context.Background(), cfgInvalid.InitSystem.Name, registry)
	if errInvalid == nil {
		t.Fatal("expected error for invalid init system")
	}

	// Empty init system triggers Detect
	cfgEmpty := &config.Config{
		InitSystem: config.InitSystemConfig{
			Name: "",
		},
	}

	// In a test environment without systemd active, Detect will fail.
	// We just want to ensure it propagates correctly.
	// Alternatively, we can register a mock that returns true.
	registry.Register("always-active", func() initsystem.InitSystem { return initsystem.NewSystemd() })
	_, errDetect := ResolveInitSystem(context.Background(), cfgEmpty.InitSystem.Name, registry)
	if errDetect != nil && errDetect.Error() != "init system detection failed: no supported init system detected" {
		t.Fatalf("unexpected error message: %v", errDetect)
	}
}

func TestCLI_Execute(t *testing.T) {
	cli := NewCLI()

	// Create a temp grub config to satisfy the command
	grubFile, err := os.CreateTemp("", "grub-*.cfg")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(grubFile.Name()) }()

	cli.RootCmd.SetArgs([]string{"options", "list", "--config", "../../config.sample.yaml", "--bootloader-path", grubFile.Name()})

	var b bytes.Buffer
	cli.RootCmd.SetOut(&b)

	err = cli.Execute()
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
