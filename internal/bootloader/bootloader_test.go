package bootloader

import (
	"testing"
)

func TestBootloaderRegistry(t *testing.T) {
	// example bootloader is registered via init() in example.go
	bl := Get("example")
	if bl == nil {
		t.Fatal("expected to retrieve 'example' bootloader, got nil")
	}

	if bl.Name() != "example" {
		t.Errorf("expected bootloader name 'example', got %s", bl.Name())
	}

	// Unknown bootloader
	blUnknown := Get("non-existent-bootloader")
	if blUnknown != nil {
		t.Errorf("expected nil for 'non-existent-bootloader', got %v", blUnknown)
	}
}

func TestDetectBootloader_Fail(t *testing.T) {
	// Temporarily clear the registry
	oldRegistry := registry
	defer func() { registry = oldRegistry }()
	registry = make(map[string]Factory)

	bl, err := Detect()
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

	bootOptions, err := bl.NewGetBootOptions("")
	if err != nil {
		t.Fatalf("expected no error from example NewGetBootOptions relative to config path, got %v", err)
	}

	if len(bootOptions) != 2 || bootOptions[0] != "Ubuntu" || bootOptions[1] != "Windows" {
		t.Errorf("expected [Ubuntu, Windows], got %v", bootOptions)
	}
}

func TestDetectBootloader(t *testing.T) {
	// 'example' always returns true for IsActive()
	bl, err := Detect()
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
