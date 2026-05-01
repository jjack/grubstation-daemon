package bootloader

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	bootOptions, err := bl.GetBootOptions(context.Background(), Config{ConfigPath: testDataPath})

	if !bl.IsActive(context.Background()) {
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

// fakeExecCommand wrappers route the exec call back to the test binary's TestHelperProcess
func fakeExecCommandSuccess(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.CommandContext(ctx, os.Args[0], cs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func fakeExecCommandFail(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", "fail"}
	cmd := exec.CommandContext(ctx, os.Args[0], cs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) > 0 && args[0] == "fail" {
		os.Exit(1)
	}
	os.Exit(0)
}

func TestGrub_Install_Success(t *testing.T) {
	bl := NewGrub()
	tempDir := t.TempDir()
	fakeScriptPath := filepath.Join(tempDir, "99_ha_remote_boot_agent")

	defer func(oldPath string, oldLook func(string) (string, error), oldCmd func(context.Context, string, ...string) *exec.Cmd) {
		hassRemoteBootAgentPath = oldPath
		execLookPath = oldLook
		execCommand = oldCmd
	}(hassRemoteBootAgentPath, execLookPath, execCommand)

	hassRemoteBootAgentPath = fakeScriptPath
	execCommand = fakeExecCommandSuccess

	// Test success using update-grub
	execLookPath = func(file string) (string, error) {
		if file == "update-grub" {
			return "/fake/update-grub", nil
		}
		return "", errors.New("not found")
	}

	err := bl.Install(context.Background(), "aa:bb:cc:dd:ee:ff", "http://hass.local:8123", "test_webhook")
	if err != nil {
		t.Fatalf("expected successful install, got %v", err)
	}

	content, _ := os.ReadFile(fakeScriptPath)
	if !strings.Contains(string(content), "http,hass.local:8123") || !strings.Contains(string(content), "aa:bb:cc:dd:ee:ff") || !strings.Contains(string(content), "test_webhook") {
		t.Errorf("template not rendered correctly: %s", string(content))
	}

	// Test fallback success using grub2-mkconfig
	execLookPath = func(file string) (string, error) {
		if file == "grub2-mkconfig" {
			return "/fake/grub2-mkconfig", nil
		}
		return "", errors.New("not found")
	}

	err = bl.Install(context.Background(), "aa:bb:cc:dd:ee:ff", "http://hass.local:8123", "test_webhook")
	if err != nil {
		t.Fatalf("expected successful install with grub2-mkconfig, got %v", err)
	}
}

func TestGrub_Install_Errors(t *testing.T) {
	bl := NewGrub()
	ctx := context.Background()

	defer func(oldPath string, oldLook func(string) (string, error), oldCmd func(context.Context, string, ...string) *exec.Cmd) {
		hassRemoteBootAgentPath = oldPath
		execLookPath = oldLook
		execCommand = oldCmd
	}(hassRemoteBootAgentPath, execLookPath, execCommand)

	// 1. Invalid URL
	err := bl.Install(ctx, "mac", "://bad-url", "test_webhook")
	if !errors.Is(err, ErrInvalidHAURL) {
		t.Fatalf("expected ErrInvalidHAURL, got %v", err)
	}

	// 2. File creation failure
	hassRemoteBootAgentPath = "/this/path/does/not/exist/99_script"
	err = bl.Install(ctx, "mac", "http://hass.local", "test_webhook")
	if err == nil || !strings.Contains(err.Error(), "failed to create grub script") {
		t.Fatalf("expected file creation error, got %v", err)
	}

	// Fix path for subsequent tests
	tempDir := t.TempDir()
	hassRemoteBootAgentPath = filepath.Join(tempDir, "99_ha_remote_boot_agent")

	// 3. No binary found in PATH
	execLookPath = func(file string) (string, error) {
		return "", errors.New("not found")
	}
	err = bl.Install(ctx, "mac", "http://hass.local", "test_webhook")
	if !errors.Is(err, ErrNoGrubTool) {
		t.Fatalf("expected ErrNoGrubTool, got %v", err)
	}

	// 4. update-grub command execution fails
	execLookPath = func(file string) (string, error) {
		if file == "update-grub" {
			return "/fake/update-grub", nil
		}
		return "", errors.New("not found")
	}
	execCommand = fakeExecCommandFail
	err = bl.Install(ctx, "mac", "http://hass.local", "test_webhook")
	if err == nil || !strings.Contains(err.Error(), "update-grub failed") {
		t.Fatalf("expected update-grub execution error, got %v", err)
	}

	// 5. grub2-mkconfig command execution fails
	execLookPath = func(file string) (string, error) {
		if file == "grub2-mkconfig" {
			return "/fake/grub2-mkconfig", nil
		}
		return "", errors.New("not found")
	}
	err = bl.Install(ctx, "mac", "http://hass.local", "test_webhook")
	if err == nil || !strings.Contains(err.Error(), "grub2-mkconfig failed") {
		t.Fatalf("expected grub2-mkconfig execution error, got %v", err)
	}
}

func TestGrubBootloader_FileNotFound(t *testing.T) {
	bl := NewGrub()
	_, err := bl.GetBootOptions(context.Background(), Config{ConfigPath: "/tmp/nonexistent/grub.cfg"})
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

	bootOptions, err := bl.GetBootOptions(context.Background(), Config{})
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

	_, err := bl.GetBootOptions(context.Background(), Config{})
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

	bootOptions, err := bl.GetBootOptions(context.Background(), Config{ConfigPath: testDataPath})
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
