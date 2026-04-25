package bootloader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGrubBootloader(t *testing.T) {
	bl := NewGrub()

	if bl.Name() != grubBootloader {
		t.Errorf("expected bootloader name 'grub', got %s", bl.Name())
	}

	// Point to the standard Go testdata directory
	testDataPath := filepath.Join("testdata", "grub.cfg")
	if _, err := os.Stat(testDataPath); os.IsNotExist(err) {
		t.Skipf("Real grub.cfg not found at %s, skipping test", testDataPath)
	}

	originalPaths := grubPaths
	defer func() { grubPaths = originalPaths }()
	grubPaths = []string{testDataPath}

	bootOptions, err := bl.GetBootOptions(testDataPath)

	if !bl.IsActive() {
		t.Error("expected grub bootloader to be logically active")
	}

	if err != nil {
		t.Fatalf("expected no error from grub GetBootOptions, got: %v", err)
	}

	wantedOptions := []string{
		"Debian GNU/Linux",
		"Advanced options for Debian GNU/Linux>Debian GNU/Linux, with Linux 6.12.74+deb13+1-amd64",
		"Advanced options for Debian GNU/Linux>Debian GNU/Linux, with Linux 6.12.74+deb13+1-amd64 (recovery mode)",
		"Advanced options for Debian GNU/Linux>Debian GNU/Linux, with Linux 6.12.73+deb13-amd64",
		"Advanced options for Debian GNU/Linux>Debian GNU/Linux, with Linux 6.12.73+deb13-amd64 (recovery mode)",
		"Windows Boot Manager (on /dev/sda1)",
		"Haiku",
		"UEFI Firmware Settings",
	}

	if len(bootOptions) != len(wantedOptions) {
		t.Errorf("expected %d OS entries, got %d", len(wantedOptions), len(bootOptions))
	} else {
		for i, opt := range bootOptions {
			if opt != wantedOptions[i] {
				t.Errorf("expected %s, got %s", wantedOptions[i], opt)
			}
		}
	}
}

func TestGrubBootloader_FileNotFound(t *testing.T) {
	bl := NewGrub()
	_, err := bl.GetBootOptions("/tmp/nonexistent/grub.cfg")
	if err == nil {
		t.Fatal("expected error on nonexistent grub config, got nil")
	}
}

func TestGrubBootloader_AutoDiscovery(t *testing.T) {
	bl := NewGrub()

	// Temporarily override the tracked paths to point to a temp dir so that the environment doesn't affect it
	tempDir := t.TempDir()
	fakeGrubPath := filepath.Join(tempDir, "grub.cfg")
	if err := os.WriteFile(fakeGrubPath, []byte("menuentry 'Arch Linux' { }"), 0o644); err != nil {
		t.Fatalf("failed to write temp grub config: %v", err)
	}

	originalPaths := grubPaths
	defer func() { grubPaths = originalPaths }()
	grubPaths = []string{fakeGrubPath}

	bootOptions, err := bl.GetBootOptions("")
	if err != nil {
		t.Fatalf("expected auto-discovery to find grub config without error, got: %v", err)
	}

	if len(bootOptions) != 1 || bootOptions[0] != "Arch Linux" {
		t.Errorf("expected 'Arch Linux' from auto-discovered file, got %v", bootOptions)
	}
}

func TestGrubBootloader_AutoDiscovery_Fail(t *testing.T) {
	bl := NewGrub()

	originalPaths := grubPaths
	defer func() { grubPaths = originalPaths }()
	grubPaths = []string{"/tmp/definitely-do-not-exist"}

	_, err := bl.GetBootOptions("")
	if err == nil {
		t.Fatal("expected failure to find any grub config")
	}
}

func TestGrubBootloader_RealConfig(t *testing.T) {
	bl := NewGrub()

	// Point to the standard Go testdata directory
	testDataPath := filepath.Join("testdata", "grub.cfg")
	if _, err := os.Stat(testDataPath); os.IsNotExist(err) {
		t.Skipf("Real grub.cfg not found at %s, skipping test", testDataPath)
	}

	bootOptions, err := bl.GetBootOptions(testDataPath)
	if err != nil {
		t.Fatalf("failed to parse real grub config: %v", err)
	}

	if len(bootOptions) == 0 {
		t.Log("Warning: No boot options found in the provided grub.cfg")
	} else {
		t.Logf("Successfully found %d boot options:", len(bootOptions))
		for _, opt := range bootOptions {
			t.Logf("  - %s", opt)
		}
	}
}
