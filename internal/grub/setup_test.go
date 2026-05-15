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
		HassGrubStationPath = oldPath
		ExecLookPath = oldLook
		ExecCommand = oldCmd
	}(HassGrubStationPath, ExecLookPath, ExecCommand)

	HassGrubStationPath = fakeScriptPath
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
		TargetMAC:       "aa:bb:cc:dd:ee:ff",
		TargetURL:       "http://hass.local:8123",
		AuthToken:       "test_webhook",
		WaitTimeSeconds: 2,
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
		TargetMAC:       "aa:bb:cc:dd:ee:ff",
		TargetURL:       "http://hass.local:8123",
		AuthToken:       "test_webhook",
		WaitTimeSeconds: 2,
	})
	if err != nil {
		t.Fatalf("expected successful install with grub2-mkconfig, got %v", err)
	}
}

func TestGrub_Setup_Errors(t *testing.T) {
	ctx := context.Background()
	g := &Grub{}

	defer func(oldPath string, oldLook func(string) (string, error), oldCmd func(context.Context, string, ...string) *exec.Cmd) {
		HassGrubStationPath = oldPath
		ExecLookPath = oldLook
		ExecCommand = oldCmd
	}(HassGrubStationPath, ExecLookPath, ExecCommand)

	// 1. Invalid URL
	err := g.Setup(ctx, SetupOptions{TargetMAC: "mac", TargetURL: "://bad-url", AuthToken: "test_webhook", WaitTimeSeconds: 2})
	if !errors.Is(err, ErrInvalidHAURL) {
		t.Fatalf("expected ErrInvalidHAURL, got %v", err)
	}

	// 2. File creation failure
	HassGrubStationPath = "/this/path/does/not/exist/99_script"
	err = g.Setup(ctx, SetupOptions{TargetMAC: "mac", TargetURL: "http://hass.local", AuthToken: "test_webhook", WaitTimeSeconds: 2})
	if err == nil || !strings.Contains(err.Error(), "failed to create grub script") {
		t.Fatalf("expected file creation error, got %v", err)
	}

	// Fix path for subsequent tests
	tempDir := t.TempDir()
	HassGrubStationPath = filepath.Join(tempDir, "99_ha_grub_os_reporter")

	// 3. No binary found in PATH
	ExecLookPath = func(file string) (string, error) {
		return "", errors.New("not found")
	}
	err = g.Setup(ctx, SetupOptions{TargetMAC: "mac", TargetURL: "http://hass.local", AuthToken: "test_webhook", WaitTimeSeconds: 2})
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
	err = g.Setup(ctx, SetupOptions{TargetMAC: "mac", TargetURL: "http://hass.local", AuthToken: "test_webhook", WaitTimeSeconds: 2})
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
	err = g.Setup(ctx, SetupOptions{TargetMAC: "mac", TargetURL: "http://hass.local", AuthToken: "test_webhook", WaitTimeSeconds: 2})
	if err == nil || !strings.Contains(err.Error(), "grub2-mkconfig failed") {
		t.Fatalf("expected grub2-mkconfig execution error, got %v", err)
	}
}

func TestGrub_Setup_TemplateErrors(t *testing.T) {
	ctx := context.Background()

	originalTemplate := grubTemplate
	defer func() { grubTemplate = originalTemplate }()
	g := &Grub{}

	// 1. Template parse error
	grubTemplate = "{{ unclosed"
	err := g.Setup(ctx, SetupOptions{TargetMAC: "mac", TargetURL: "http://hass.local", AuthToken: "test_webhook", WaitTimeSeconds: 2})
	if err == nil || !strings.Contains(err.Error(), "failed to parse grub template") {
		t.Fatalf("expected template parse error, got %v", err)
	}

	// 2. Template execute error
	// Accessing a nonexistent field on a string will cause template execution to fail
	grubTemplate = "{{ .Host.NonExistentField }}"
	err = g.Setup(ctx, SetupOptions{TargetMAC: "mac", TargetURL: "http://hass.local", AuthToken: "test_webhook", WaitTimeSeconds: 2})
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

func TestGenerateWaitList(t *testing.T) {
	if got := generateWaitList(0); got != "1" {
		t.Errorf("expected 1, got %q", got)
	}
	if got := generateWaitList(-1); got != "1" {
		t.Errorf("expected 1, got %q", got)
	}
	if got := generateWaitList(3); got != "1 2 3" {
		t.Errorf("expected '1 2 3', got %q", got)
	}
}

func TestGrub_Uninstall(t *testing.T) {
	tempDir := t.TempDir()
	HassGrubStationPath = tempDir + "/99_grubstation"

	// Pre-create file
	_ = os.WriteFile(HassGrubStationPath, []byte(""), 0o755)

	oldExecLookPath := ExecLookPath
	oldExecCommand := ExecCommand
	defer func() {
		ExecLookPath = oldExecLookPath
		ExecCommand = oldExecCommand
	}()

	ExecLookPath = func(file string) (string, error) {
		if file == "update-grub" {
			return "/bin/true", nil
		}
		return "", errors.New("not found")
	}
	ExecCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	g := &Grub{}
	err := g.Uninstall(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(HassGrubStationPath); !os.IsNotExist(err) {
		t.Error("expected grub script to be removed")
	}
}

func TestGrub_Uninstall_NoFile(t *testing.T) {
	tempDir := t.TempDir()
	HassGrubStationPath = tempDir + "/non-existent"

	g := &Grub{}
	err := g.Uninstall(context.Background())
	if err != nil {
		t.Fatalf("expected no error when file is already gone, got %v", err)
	}
}

func TestGrub_Uninstall_RemoveError(t *testing.T) {
	// Use a non-empty directory to cause remove error
	tempDir := t.TempDir()
	HassGrubStationPath = tempDir
	_ = os.WriteFile(filepath.Join(tempDir, "keep"), []byte(""), 0o644)

	g := &Grub{}
	err := g.Uninstall(context.Background())
	if err == nil {
		t.Fatal("expected error when removing a non-empty directory, got nil")
	}
}

func TestGrub_Uninstall_Grub2Mkconfig(t *testing.T) {
	tempDir := t.TempDir()
	HassGrubStationPath = tempDir + "/99_grubstation"

	oldExecLookPath := ExecLookPath
	oldExecCommand := ExecCommand
	defer func() {
		ExecLookPath = oldExecLookPath
		ExecCommand = oldExecCommand
	}()

	ExecLookPath = func(file string) (string, error) {
		if file == "grub2-mkconfig" {
			return "/bin/true", nil
		}
		return "", errors.New("not found")
	}
	ExecCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	g := &Grub{}
	err := g.Uninstall(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGrub_Uninstall_UpdateGrubError(t *testing.T) {
	oldExecLookPath := ExecLookPath
	oldExecCommand := ExecCommand
	defer func() {
		ExecLookPath = oldExecLookPath
		ExecCommand = oldExecCommand
	}()

	ExecLookPath = func(file string) (string, error) {
		if file == "update-grub" {
			return "/bin/false", nil
		}
		return "", errors.New("not found")
	}
	ExecCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	g := &Grub{}
	err := g.Uninstall(context.Background())
	if err == nil || !strings.Contains(err.Error(), "update-grub failed") {
		t.Errorf("expected update-grub failure, got %v", err)
	}
}

func TestGrub_Uninstall_Grub2MkconfigError(t *testing.T) {
	oldExecLookPath := ExecLookPath
	oldExecCommand := ExecCommand
	defer func() {
		ExecLookPath = oldExecLookPath
		ExecCommand = oldExecCommand
	}()

	ExecLookPath = func(file string) (string, error) {
		if file == "grub2-mkconfig" {
			return "/bin/false", nil
		}
		return "", errors.New("not found")
	}
	ExecCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	g := &Grub{}
	err := g.Uninstall(context.Background())
	if err == nil || !strings.Contains(err.Error(), "grub2-mkconfig failed") {
		t.Errorf("expected grub2-mkconfig failure, got %v", err)
	}
}
