package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
)

type mockListBootloader struct{}

func (m *mockListBootloader) Name() string                      { return "example" }
func (m *mockListBootloader) IsActive(ctx context.Context) bool { return true }
func (m *mockListBootloader) GetBootOptions(ctx context.Context, cfg bootloader.Config) ([]string, error) {
	return []string{"Ubuntu", "Windows"}, nil
}
func (m *mockListBootloader) Install(ctx context.Context, macAddress, haURL string) error { return nil }

func TestGetBootOptionsCommand(t *testing.T) {
	cfg := &config.Config{
		Bootloader: config.BootloaderConfig{
			Name: "example",
		},
	}

	registry := bootloader.NewRegistry()
	registry.Register("example", func() bootloader.Bootloader { return &mockListBootloader{} })

	deps := &CommandDeps{Config: cfg, BootloaderRegistry: registry}
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

type mockListBootloaderErr struct{}

func (m *mockListBootloaderErr) Name() string                      { return "err" }
func (m *mockListBootloaderErr) IsActive(ctx context.Context) bool { return true }
func (m *mockListBootloaderErr) GetBootOptions(ctx context.Context, cfg bootloader.Config) ([]string, error) {
	return nil, errors.New("mock error")
}

func (m *mockListBootloaderErr) Install(ctx context.Context, macAddress, haURL string) error {
	return nil
}

func TestGetBootOptionsCommand_BootloaderError(t *testing.T) {
	cfg := &config.Config{
		Bootloader: config.BootloaderConfig{
			Name: "err",
		},
	}

	registry := bootloader.NewRegistry()
	registry.Register("err", func() bootloader.Bootloader { return &mockListBootloaderErr{} })

	deps := &CommandDeps{Config: cfg, BootloaderRegistry: registry}
	cmd := NewListCmd(deps)
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error from GetBootOptions, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get boot options") {
		t.Errorf("unexpected error message: %v", err)
	}
}

type mockListBootloaderEmpty struct{}

func (m *mockListBootloaderEmpty) Name() string                      { return "empty" }
func (m *mockListBootloaderEmpty) IsActive(ctx context.Context) bool { return true }
func (m *mockListBootloaderEmpty) GetBootOptions(ctx context.Context, cfg bootloader.Config) ([]string, error) {
	return []string{}, nil
}

func (m *mockListBootloaderEmpty) Install(ctx context.Context, macAddress, haURL string) error {
	return nil
}

func TestGetBootOptionsCommand_EmptyOptions(t *testing.T) {
	cfg := &config.Config{
		Bootloader: config.BootloaderConfig{
			Name: "empty",
		},
	}

	registry := bootloader.NewRegistry()
	registry.Register("empty", func() bootloader.Bootloader { return &mockListBootloaderEmpty{} })

	deps := &CommandDeps{Config: cfg, BootloaderRegistry: registry}
	cmd := NewListCmd(deps)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_ = cmd.Execute()

	_ = w.Close()
	os.Stdout = oldStdout

	out, _ := io.ReadAll(r)
	if !strings.Contains(string(out), "(None found)") {
		t.Errorf("output missing '(None found)': %s", string(out))
	}
}

func TestGetBootOptionsCommand_UnknownBootloader(t *testing.T) {
	cfg := &config.Config{
		Bootloader: config.BootloaderConfig{
			Name: "unknown",
		},
	}

	registry := bootloader.NewRegistry() // Empty registry for unknown bootloader

	deps := &CommandDeps{Config: cfg, BootloaderRegistry: registry}
	cmd := NewListCmd(deps)
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error for unknown bootloader, got nil")
	}
	if !strings.Contains(err.Error(), "specified bootloader unknown not supported") {
		t.Errorf("unexpected error message: %v", err)
	}
}
