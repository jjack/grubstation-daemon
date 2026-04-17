package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/jjack/ha-remote-boot-agent/internal/config"
)

func TestNewCLI(t *testing.T) {
	cli := NewCLI()
	if cli == nil {
		t.Fatal("expected pointer to CLI, got nil")
	}
	if cli.RootCmd == nil {
		t.Fatal("expected RootCmd to be initialized")
	}
	if cli.RootCmd.Use != "ha-remote-boot-agent" {
		t.Errorf("expected use 'ha-remote-boot-agent', got %s", cli.RootCmd.Use)
	}
}

func TestResolveBootloader(t *testing.T) {
	cfg := &config.Config{
		Bootloader: config.BootloaderConfig{
			Name: "example",
		},
	}

	bl, err := ResolveBootloader(cfg)
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
	_, errInvalid := ResolveBootloader(cfgInvalid)
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
	blDetect, errDetect := ResolveBootloader(cfgEmpty)
	if errDetect != nil {
		t.Fatalf("expected no error detecting, got %v", errDetect)
	}
	if blDetect == nil {
		t.Fatal("expected detected bootloader to not be nil")
	}
}

func TestCLI_Execute(t *testing.T) {
	cli := NewCLI()

	// Create a temp config file
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	_, _ = f.Write([]byte("bootloader:\n  name: example\n"))
	_ = f.Close()

	cli.RootCmd.SetArgs([]string{"list", "--config", f.Name()})

	var b bytes.Buffer
	cli.RootCmd.SetOut(&b)

	err = cli.Execute()
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
}
