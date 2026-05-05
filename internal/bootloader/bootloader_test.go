package bootloader

import (
	"context"
	"testing"
)

type mockBootloader struct{}

func (m *mockBootloader) IsActive(ctx context.Context) bool { return true }
func (m *mockBootloader) GetBootOptions(ctx context.Context, cfg Config) ([]string, error) {
	return []string{"Ubuntu", "Windows"}, nil
}
func (m *mockBootloader) Name() string { return "example" }
func (m *mockBootloader) Setup(ctx context.Context, macAddress, haURL, webhookID string) error {
	return nil
}

func (m *mockBootloader) DiscoverConfigPath(ctx context.Context) (string, error) {
	return "/path/to/example.cfg", nil
}

func TestBootloaderRegistry(t *testing.T) {
	registry := NewRegistry()
	registry.Register("example", func() Bootloader { return &mockBootloader{} })

	bl := registry.Get("example")
	if bl == nil {
		t.Fatal("expected to retrieve 'example' bootloader, got nil")
	}

	if bl.Name() != "example" {
		t.Errorf("expected bootloader name 'example', got %s", bl.Name())
	}

	// Unknown bootloader
	blUnknown := registry.Get("non-existent-bootloader")
	if blUnknown != nil {
		t.Errorf("expected nil for 'non-existent-bootloader', got %v", blUnknown)
	}
}

func TestDetectBootloader_Fail(t *testing.T) {
	// Empty registry
	registry := NewRegistry()

	bl, err := registry.Detect(context.Background())
	if err == nil {
		t.Fatal("expected error detecting bootloader with empty registry, got nil")
	}
	if bl != nil {
		t.Fatal("expected nil bootloader on detect fail")
	}
}

func TestSupportedBootloaders(t *testing.T) {
	registry := NewRegistry()
	registry.Register("zebra", func() Bootloader { return &mockBootloader{} })
	registry.Register("alpha", func() Bootloader { return &mockBootloader{} })

	supported := registry.SupportedBootloaders()
	if len(supported) != 2 {
		t.Fatalf("expected 2 supported bootloaders, got %d", len(supported))
	}
	if supported[0] != "alpha" || supported[1] != "zebra" {
		t.Errorf("expected [alpha, zebra] in sorted order, got %v", supported)
	}
}

func TestMockBootloader(t *testing.T) {
	bl := &mockBootloader{}

	if !bl.IsActive(context.Background()) {
		t.Error("expected example bootloader to be active")
	}

	bootOptions, err := bl.GetBootOptions(context.Background(), Config{})
	if err != nil {
		t.Fatalf("expected no error from example GetBootOptions relative to config path, got %v", err)
	}

	if len(bootOptions) != 2 || bootOptions[0] != "Ubuntu" || bootOptions[1] != "Windows" {
		t.Errorf("expected [Ubuntu, Windows], got %v", bootOptions)
	}
}

func TestDetectBootloader(t *testing.T) {
	registry := NewRegistry()
	registry.Register("example", func() Bootloader { return &mockBootloader{} })

	// 'example' always returns true for IsActive()
	bl, err := registry.Detect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error detecting bootloader: %v", err)
	}

	if bl == nil {
		t.Fatal("expected bootloader to be detected, got nil")
	}

	// Verify one of the active ones is chosen
	if bl.Name() == "" {
		t.Error("expected detected bootloader to have a name")
	}
}
