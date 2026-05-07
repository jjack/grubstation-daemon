package grub

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrub(t *testing.T) {
	// Point to the standard Go testdata directory
	testDataPath := filepath.Join("testdata", "grub.cfg")
	if _, err := os.Stat(testDataPath); os.IsNotExist(err) {
		t.Skipf("Real grub.cfg not found at %s, skipping test", testDataPath)
	}

	originalPaths := configPaths
	defer func() { configPaths = originalPaths }()
	configPaths = []string{testDataPath}

	g := &Grub{ConfigPath: testDataPath}
	bootOptions, err := g.GetBootOptions(context.Background())
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

func TestGrub_Setup_Success(t *testing.T) {
	tempDir := t.TempDir()
	fakeScriptPath := filepath.Join(tempDir, "99_ha_grub_os_reporter")

	defer func(oldPath string, oldLook func(string) (string, error), oldCmd func(context.Context, string, ...string) *exec.Cmd) {
		HassGrubOSReporterPath = oldPath
		ExecLookPath = oldLook
		ExecCommand = oldCmd
	}(HassGrubOSReporterPath, ExecLookPath, ExecCommand)

	HassGrubOSReporterPath = fakeScriptPath
	ExecCommand = fakeExecCommandSuccess

	// Test success using update-grub
	ExecLookPath = func(file string) (string, error) {
		if file == "update-grub" {
			return "/fake/update-grub", nil
		}
		return "", errors.New("not found")
	}

	g := &Grub{}
	err := g.Setup(context.Background(), SetupOptions{
		TargetMAC: "aa:bb:cc:dd:ee:ff",
		TargetURL: "http://hass.local:8123",
		AuthToken: "test_webhook",
	})
	if err != nil {
		t.Fatalf("expected successful install, got %v", err)
	}

	content, _ := os.ReadFile(fakeScriptPath)
	if !strings.Contains(string(content), "http,hass.local:8123") || !strings.Contains(string(content), "aa:bb:cc:dd:ee:ff") || !strings.Contains(string(content), "test_webhook") {
		t.Errorf("template not rendered correctly: %s", string(content))
	}

	// Test fallback success using grub2-mkconfig
	ExecLookPath = func(file string) (string, error) {
		if file == "grub2-mkconfig" {
			return "/fake/grub2-mkconfig", nil
		}
		return "", errors.New("not found")
	}

	err = g.Setup(context.Background(), SetupOptions{
		TargetMAC: "aa:bb:cc:dd:ee:ff",
		TargetURL: "http://hass.local:8123",
		AuthToken: "test_webhook",
	})
	if err != nil {
		t.Fatalf("expected successful install with grub2-mkconfig, got %v", err)
	}
}

func TestGrub_Setup_Errors(t *testing.T) {
	ctx := context.Background()
	g := &Grub{}

	defer func(oldPath string, oldLook func(string) (string, error), oldCmd func(context.Context, string, ...string) *exec.Cmd) {
		HassGrubOSReporterPath = oldPath
		ExecLookPath = oldLook
		ExecCommand = oldCmd
	}(HassGrubOSReporterPath, ExecLookPath, ExecCommand)

	// 1. Invalid URL
	err := g.Setup(ctx, SetupOptions{TargetMAC: "mac", TargetURL: "://bad-url", AuthToken: "test_webhook"})
	if !errors.Is(err, ErrInvalidHAURL) {
		t.Fatalf("expected ErrInvalidHAURL, got %v", err)
	}

	// 2. File creation failure
	HassGrubOSReporterPath = "/this/path/does/not/exist/99_script"
	err = g.Setup(ctx, SetupOptions{TargetMAC: "mac", TargetURL: "http://hass.local", AuthToken: "test_webhook"})
	if err == nil || !strings.Contains(err.Error(), "failed to create grub script") {
		t.Fatalf("expected file creation error, got %v", err)
	}

	// Fix path for subsequent tests
	tempDir := t.TempDir()
	HassGrubOSReporterPath = filepath.Join(tempDir, "99_ha_grub_os_reporter")

	// 3. No binary found in PATH
	ExecLookPath = func(file string) (string, error) {
		return "", errors.New("not found")
	}
	err = g.Setup(ctx, SetupOptions{TargetMAC: "mac", TargetURL: "http://hass.local", AuthToken: "test_webhook"})
	if !errors.Is(err, ErrNoGrubTool) {
		t.Fatalf("expected ErrNoGrubTool, got %v", err)
	}

	// 4. update-grub command execution fails
	ExecLookPath = func(file string) (string, error) {
		if file == "update-grub" {
			return "/fake/update-grub", nil
		}
		return "", errors.New("not found")
	}
	ExecCommand = fakeExecCommandFail
	err = g.Setup(ctx, SetupOptions{TargetMAC: "mac", TargetURL: "http://hass.local", AuthToken: "test_webhook"})
	if err == nil || !strings.Contains(err.Error(), "update-grub failed") {
		t.Fatalf("expected update-grub execution error, got %v", err)
	}

	// 5. grub2-mkconfig command execution fails
	ExecLookPath = func(file string) (string, error) {
		if file == "grub2-mkconfig" {
			return "/fake/grub2-mkconfig", nil
		}
		return "", errors.New("not found")
	}
	err = g.Setup(ctx, SetupOptions{TargetMAC: "mac", TargetURL: "http://hass.local", AuthToken: "test_webhook"})
	if err == nil || !strings.Contains(err.Error(), "grub2-mkconfig failed") {
		t.Fatalf("expected grub2-mkconfig execution error, got %v", err)
	}
}

func TestGrub_FileNotFound(t *testing.T) {
	g := &Grub{ConfigPath: "/tmp/nonexistent/grub.cfg"}
	_, err := g.GetBootOptions(context.Background())
	if err == nil {
		t.Fatal("expected error on nonexistent grub config, got nil")
	}
}

func TestGrub_AutoDiscovery(t *testing.T) {
	// Temporarily override the tracked paths to point to a temp dir so that the environment doesn't affect it
	tempDir := t.TempDir()
	fakeGrubPath := filepath.Join(tempDir, "grub.cfg")
	if err := os.WriteFile(fakeGrubPath, []byte("menuentry 'Arch Linux' { }"), 0o644); err != nil {
		t.Fatalf("failed to write temp grub config: %v", err)
	}

	originalPaths := configPaths
	defer func() { configPaths = originalPaths }()
	configPaths = []string{fakeGrubPath}

	g := &Grub{}
	bootOptions, err := g.GetBootOptions(context.Background())
	if err != nil {
		t.Fatalf("expected auto-discovery to find grub config without error, got: %v", err)
	}

	if len(bootOptions) != 1 || bootOptions[0] != "Arch Linux" {
		t.Errorf("expected 'Arch Linux' from auto-discovered file, got %v", bootOptions)
	}
}

func TestGrub_AutoDiscovery_Fail(t *testing.T) {
	originalPaths := configPaths
	defer func() { configPaths = originalPaths }()
	configPaths = []string{"/tmp/definitely-do-not-exist"}

	g := &Grub{}
	_, err := g.GetBootOptions(context.Background())
	if err == nil {
		t.Fatal("expected failure to find any grub config")
	}
}

func TestGrub_RealConfig(t *testing.T) {
	// Point to the standard Go testdata directory
	testDataPath := filepath.Join("testdata", "grub.cfg")
	if _, err := os.Stat(testDataPath); os.IsNotExist(err) {
		t.Skipf("Real grub.cfg not found at %s, skipping test", testDataPath)
	}

	g := &Grub{ConfigPath: testDataPath}
	bootOptions, err := g.GetBootOptions(context.Background())
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

func TestCountStructuralBraces(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		opens  int
		closes int
	}{
		{"simple menuentry", "menuentry 'Linux' {", 1, 0},
		{"closing brace", "}", 0, 1},
		{"comment", "# this is a comment { }", 0, 0},
		{"double quotes", "menuentry \"with a brace { inside\" {", 1, 0},
		{"single quotes", "menuentry 'with a brace { inside' {", 1, 0},
		{"escaped braces", "escaped \\{ \\} {", 1, 0},
		{"nested braces", "nested { { } }", 2, 2},
		{"hash inside quotes", "echo 'hash # inside quotes' {", 1, 0},
		{"quote inside quote", "echo \"it's nice\" {", 1, 0},
		{"escaped quote", "echo 'it\\'s nice' {", 1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opens, closes := countStructuralBraces(tt.line)
			if opens != tt.opens || closes != tt.closes {
				t.Errorf("countStructuralBraces(%q) = %d, %d; want %d, %d", tt.line, opens, closes, tt.opens, tt.closes)
			}
		})
	}
}

func TestGrub_Discover(t *testing.T) {
	tempDir := t.TempDir()
	fakeGrubPath := filepath.Join(tempDir, "grub.cfg")
	if err := os.WriteFile(fakeGrubPath, []byte(""), 0o644); err != nil {
		t.Fatalf("failed to write temp grub config: %v", err)
	}

	originalPaths := configPaths
	defer func() { configPaths = originalPaths }()

	// Test success cases
	configPaths = []string{fakeGrubPath}

	g := &Grub{}
	path, err := g.DiscoverConfigPath(context.Background())
	if err != nil {
		t.Errorf("expected no error from DiscoverConfigPath, got %v", err)
	}
	if path != fakeGrubPath {
		t.Errorf("expected discovered path %s, got %s", fakeGrubPath, path)
	}

	// Test error case
	configPaths = []string{"/nonexistent/grub.cfg"}
	_, err = g.DiscoverConfigPath(context.Background())
	if !errors.Is(err, ErrConfigNotFound) {
		t.Errorf("expected ErrConfigNotFound, got %v", err)
	}
}

func TestGrub_GetBootOptions_PermissionDenied(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "unreadable.cfg")
	if err := os.WriteFile(tempFile, []byte("test"), 0o200); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	g := &Grub{ConfigPath: tempFile}
	_, err := g.GetBootOptions(context.Background())
	if err == nil {
		t.Skip("expected error reading write-only file (running as root?)")
	}
	if !strings.Contains(err.Error(), "permission denied reading grub config") {
		if strings.Contains(err.Error(), "failed to open grub config") {
			t.Skipf("Got non-permission error during file open, skipping strict permission check: %v", err)
		}
		t.Errorf("expected permission denied error, got: %v", err)
	}
}

func TestGetBootOptions_ScannerError(t *testing.T) {
	// Create a file with a line longer than the buffer to trigger a scanner error.
	// The buffer has a max capacity of 1MB. We'll make a line longer than that.
	const maxBufferCapacity = 1024 * 1024 // 1MB
	longLine := strings.Repeat("a", maxBufferCapacity+1)
	content := "menuentry 'Long Line OS' {\n" + longLine + "\n}\n"

	tmpfile, err := os.CreateTemp(t.TempDir(), "grub.cfg")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := tmpfile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	g := &Grub{ConfigPath: tmpfile.Name()}
	_, err = g.GetBootOptions(context.Background())

	if err == nil {
		t.Fatal("expected an error from scanner, but got nil")
	}

	if !strings.Contains(err.Error(), "error reading grub config") {
		t.Errorf("expected error message to contain 'error reading grub config', got: %v", err)
	}
}

func TestGrub_Setup_TemplateErrors(t *testing.T) {
	ctx := context.Background()

	originalTemplate := grubTemplate
	defer func() { grubTemplate = originalTemplate }()
	g := &Grub{}

	// 1. Template parse error
	grubTemplate = "{{ unclosed"
	err := g.Setup(ctx, SetupOptions{TargetMAC: "mac", TargetURL: "http://hass.local", AuthToken: "test_webhook"})
	if err == nil || !strings.Contains(err.Error(), "failed to parse grub template") {
		t.Fatalf("expected template parse error, got %v", err)
	}

	// 2. Template execute error
	// Accessing a nonexistent field on a string will cause template execution to fail
	grubTemplate = "{{ .Host.NonExistentField }}"
	err = g.Setup(ctx, SetupOptions{TargetMAC: "mac", TargetURL: "http://hass.local", AuthToken: "test_webhook"})
	if err == nil || !strings.Contains(err.Error(), "failed to execute grub template") {
		t.Fatalf("expected template execute error, got %v", err)
	}
}

func TestGrub_SetupWarning(t *testing.T) {
	g := &Grub{}
	warning := g.SetupWarning()
	if !strings.Contains(warning, "troubleshoot your GRUB network settings") {
		t.Errorf("expected warning to mention troubleshooting, got: %s", warning)
	}
}
