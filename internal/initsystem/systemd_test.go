package initsystem

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemd_IsActive(t *testing.T) {
	sys := NewSystemd()
	if sys.Name() != "systemd" {
		t.Errorf("expected name 'systemd', got %s", sys.Name())
	}

	tempDir := t.TempDir()
	fakeSystemdDir := filepath.Join(tempDir, "system")
	if err := os.MkdirAll(fakeSystemdDir, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	oldSystemdDir := systemdDir
	defer func() { systemdDir = oldSystemdDir }()

	systemdDir = fakeSystemdDir
	if !sys.IsActive(context.Background()) {
		t.Error("expected systemd to be active with mock directory")
	}

	systemdDir = filepath.Join(tempDir, "nonexistent")
	if sys.IsActive(context.Background()) {
		t.Error("expected systemd to be inactive with nonexistent directory")
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

func fakeExecCommandFailEnable(ctx context.Context, command string, args ...string) *exec.Cmd {
	if len(args) > 0 && args[0] == "enable" {
		return fakeExecCommandFail(ctx, command, args...)
	}
	return fakeExecCommandSuccess(ctx, command, args...)
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

func TestSystemd_Install_Success(t *testing.T) {
	defer func(oe func() (string, error), ow func(string, []byte, os.FileMode) error, ec func(context.Context, string, ...string) *exec.Cmd) {
		osExecutable = oe
		osWriteFile = ow
		execCommand = ec
	}(osExecutable, osWriteFile, execCommand)

	oldSystemdTemplate := systemdTemplate
	systemdTemplate = "[Service]\nExecStart={{ .ExecPath }} push --config {{ .ConfigPath }}"
	defer func() { systemdTemplate = oldSystemdTemplate }()

	osExecutable = func() (string, error) { return "/fake/bin/remote-boot-agent", nil }
	osWriteFile = func(name string, data []byte, perm os.FileMode) error { return nil }
	execCommand = fakeExecCommandSuccess

	sys := NewSystemd()
	err := sys.Setup(context.Background(), "/fake/config.yaml")
	if err != nil {
		t.Fatalf("expected successful install, got %v", err)
	}
}

func TestSystemd_Install_Errors(t *testing.T) {
	defer func(oe func() (string, error), ow func(string, []byte, os.FileMode) error, ec func(context.Context, string, ...string) *exec.Cmd) {
		osExecutable = oe
		osWriteFile = ow
		execCommand = ec
	}(osExecutable, osWriteFile, execCommand)

	oldSystemdTemplate := systemdTemplate
	systemdTemplate = "[Service]\nExecStart={{ .ExecPath }} push --config {{ .ConfigPath }}"
	defer func() { systemdTemplate = oldSystemdTemplate }()

	sys := NewSystemd()
	ctx := context.Background()

	// 1. osExecutable error
	osExecutable = func() (string, error) { return "", errors.New("mock exec err") }
	err := sys.Setup(ctx, "config.yaml")
	if err == nil || !strings.Contains(err.Error(), "mock exec err") {
		t.Fatalf("expected mock exec err, got %v", err)
	}

	// 2. osWriteFile error
	osExecutable = func() (string, error) { return "/fake/bin", nil }
	osWriteFile = func(name string, data []byte, perm os.FileMode) error { return errors.New("mock write err") }
	err = sys.Setup(ctx, "config.yaml")
	if err == nil || !strings.Contains(err.Error(), "failed to write systemd service file") {
		t.Fatalf("expected write file error, got %v", err)
	}

	// 3. daemon-reload error
	osWriteFile = func(name string, data []byte, perm os.FileMode) error { return nil }
	execCommand = fakeExecCommandFail
	err = sys.Setup(ctx, "config.yaml")
	if err == nil || !strings.Contains(err.Error(), "failed to reload systemd daemon") {
		t.Fatalf("expected daemon-reload error, got %v", err)
	}

	// 4. enable error
	execCommand = fakeExecCommandFailEnable
	err = sys.Setup(ctx, "config.yaml")
	if err == nil || !strings.Contains(err.Error(), "failed to enable systemd service") {
		t.Fatalf("expected enable error, got %v", err)
	}
}
