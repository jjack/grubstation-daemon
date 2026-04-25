package cli

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
)

type mockListBootloader struct{}

func (m *mockListBootloader) Name() string   { return "example" }
func (m *mockListBootloader) IsActive() bool { return true }
func (m *mockListBootloader) GetBootOptions(cfg bootloader.Config) ([]string, error) {
	return []string{"Ubuntu", "Windows"}, nil
}

func TestGetBootOptionsCommand(t *testing.T) {
	cfg := &config.Config{
		Bootloader: config.BootloaderConfig{
			Name: "example",
		},
	}

	registry := bootloader.NewRegistry()
	registry.Register("example", func() bootloader.Bootloader { return &mockListBootloader{} })

	deps := &CommandDeps{Config: cfg, Registry: registry}
	cmd := NewListCmd(deps)

	// Intercept stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmd.Execute()

	_ = w.Close()
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
		t.Errorf("output missing boot option 'Ubuntu': %s", output)
	}
	if !strings.Contains(output, "- Windows") {
		t.Errorf("output missing boot option 'Windows': %s", output)
	}
}

func TestGetBootOptionsCommand_UnknownBootloader(t *testing.T) {
	cfg := &config.Config{
		Bootloader: config.BootloaderConfig{
			Name: "unknown",
		},
	}

	registry := bootloader.NewRegistry() // Empty registry for unknown bootloader

	deps := &CommandDeps{Config: cfg, Registry: registry}
	cmd := NewListCmd(deps)
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error for unknown bootloader, got nil")
	}
	if !strings.Contains(err.Error(), "specified bootloader unknown not supported") {
		t.Errorf("unexpected error message: %v", err)
	}
}
