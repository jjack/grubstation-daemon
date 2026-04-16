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

	if !bl.IsActive() {
		t.Error("expected grub bootloader to be logically active (based on stub IsActive implementation)")
	}

	tempDir := t.TempDir()
	grubConfigPath := filepath.Join(tempDir, "grub.cfg")

	grubContent := `
# some comment
menuentry 'Ubuntu' --class ubuntu --class gnu-linux --class gnu --class os $menuentry_id_option 'gnulinux-simple-uuid' {
	recordfail
}
menuentry "Windows 10" {
	insmod part_gpt
}
submenu 'Advanced options for Ubuntu' {
	menuentry 'Ubuntu, with Linux 5.15.0-generic' {
		recordfail
	}
}
`
	if err := os.WriteFile(grubConfigPath, []byte(grubContent), 0o644); err != nil {
		t.Fatalf("failed to write temp grub config: %v", err)
	}

	bootOptions, err := bl.GetBootOptions(grubConfigPath)
	if err != nil {
		t.Fatalf("expected no error from grub GetBootOptions, got: %v", err)
	}

	if len(bootOptions) != 3 {
		t.Errorf("expected 3 OS entries, got %d", len(bootOptions))
	} else {
		if bootOptions[0] != "Ubuntu" {
			t.Errorf("expected 'Ubuntu', got '%s'", bootOptions[0])
		}
		if bootOptions[1] != "Windows 10" {
			t.Errorf("expected 'Windows 10', got '%s'", bootOptions[1])
		}
		if bootOptions[2] != "Ubuntu, with Linux 5.15.0-generic" {
			t.Errorf("expected 'Ubuntu, with Linux 5.15.0-generic', got '%s'", bootOptions[2])
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
