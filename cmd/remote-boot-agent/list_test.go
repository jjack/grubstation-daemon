package main

import (
	"io"
	"os"
	"strings"
	"testing"

	_ "github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
)

func TestGetOSListCommand(t *testing.T) {
	cli := &CLI{
		Config: &config.Config{
			Bootloader: config.BootloaderConfig{
				Name: "example",
			},
		},
	}

	cmd := GetOSList(cli)

	// Intercept stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out, _ := io.ReadAll(r)
	output := string(out)

	if !strings.Contains(output, "Bootloader: example") {
		t.Errorf("output missing bootloader name: %s", output)
	}
	if !strings.Contains(output, "- Ubuntu") {
		t.Errorf("output missing OS 'Ubuntu': %s", output)
	}
	if !strings.Contains(output, "- Windows") {
		t.Errorf("output missing OS 'Windows': %s", output)
	}
}

func TestGetOSListCommand_UnknownBootloader(t *testing.T) {
	cli := &CLI{
		Config: &config.Config{
			Bootloader: config.BootloaderConfig{
				Name: "unknown",
			},
		},
	}

	cmd := GetOSList(cli)
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error for unknown bootloader, got nil")
	}
	if !strings.Contains(err.Error(), "specified bootloader unknown not supported") {
		t.Errorf("unexpected error message: %v", err)
	}
}
