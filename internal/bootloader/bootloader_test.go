package bootloader

import (
	"testing"
)

func TestBootloaderRegistry(t *testing.T) {
	registry := NewRegistry()
	registry.Register("example", NewExample)

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

	bl, err := registry.Detect()
	if err == nil {
		t.Fatal("expected error detecting bootloader with empty registry, got nil")
	}
	if bl != nil {
		t.Fatal("expected nil bootloader on detect fail")
	}
}

func TestExampleBootloader(t *testing.T) {
	bl := NewExample()

	if !bl.IsActive() {
		t.Error("expected example bootloader to be active")
	}

	bootOptions, err := bl.GetBootOptions("")
	if err != nil {
		t.Fatalf("expected no error from example GetBootOptions relative to config path, got %v", err)
	}

	if len(bootOptions) != 2 || bootOptions[0] != "Ubuntu" || bootOptions[1] != "Windows" {
		t.Errorf("expected [Ubuntu, Windows], got %v", bootOptions)
	}
}

func TestDetectBootloader(t *testing.T) {
	registry := NewRegistry()
	registry.Register("example", NewExample)

	// 'example' always returns true for IsActive()
	bl, err := registry.Detect()
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
