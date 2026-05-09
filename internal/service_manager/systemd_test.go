//go:build linux

package service_manager

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
)

func TestSystemd(t *testing.T) {
	oldExe := osExecutable
	oldWrite := osWriteFile
	oldCmd := execCommand
	oldDir := systemdDir
	oldPath := systemdServicePath
	defer func() {
		osExecutable = oldExe
		osWriteFile = oldWrite
		execCommand = oldCmd
		systemdDir = oldDir
		systemdServicePath = oldPath
	}()

	s := NewSystemd()

	t.Run("Basic", func(t *testing.T) {
		if s.Name() != "systemd" {
			t.Errorf("expected systemd, got %s", s.Name())
		}

		systemdDir = t.TempDir()
		if !s.IsActive(context.Background()) {
			t.Error("expected active when systemd directory exists")
		}
	})

	t.Run("Install_Success", func(t *testing.T) {
		osExecutable = func() (string, error) { return "/app", nil }
		osWriteFile = func(name string, data []byte, perm os.FileMode) error { return nil }
		execCommand = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			return exec.Command("true")
		}

		if err := s.Install(context.Background(), "config.yaml"); err != nil {
			t.Errorf("Install failed: %v", err)
		}
	})

	t.Run("Install_Errors", func(t *testing.T) {
		osExecutable = func() (string, error) { return "", errors.New("exe fail") }
		if err := s.Install(context.Background(), "cfg"); err == nil {
			t.Error("expected error on executable fail")
		}

		osExecutable = func() (string, error) { return "/app", nil }
		osWriteFile = func(name string, data []byte, perm os.FileMode) error { return errors.New("write fail") }
		if err := s.Install(context.Background(), "cfg"); err == nil {
			t.Error("expected error on write fail")
		}

		osWriteFile = func(name string, data []byte, perm os.FileMode) error { return nil }
		execCommand = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			return exec.Command("false") // CombinedOutput returns error on non-zero exit
		}
		if err := s.Install(context.Background(), "cfg"); err == nil {
			t.Error("expected error on command execution fail")
		}
	})

	t.Run("Uninstall", func(t *testing.T) {
		systemdServicePath = t.TempDir() + "/svc"
		_ = os.WriteFile(systemdServicePath, []byte(""), 0o644)

		execCommand = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			return exec.Command("true")
		}

		if err := s.Uninstall(context.Background()); err != nil {
			t.Errorf("Uninstall failed: %v", err)
		}

		// Reload fail
		execCommand = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			if len(arg) > 0 && arg[0] == "daemon-reload" {
				return exec.Command("false")
			}
			return exec.Command("true")
		}
		if err := s.Uninstall(context.Background()); err == nil {
			t.Error("expected error on reload failure")
		}
	})

	t.Run("StartStop", func(t *testing.T) {
		execCommand = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			return exec.Command("true")
		}
		_ = s.Start(context.Background())
		_ = s.Stop(context.Background())

		execCommand = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			return exec.Command("false")
		}
		if err := s.Start(context.Background()); err == nil {
			t.Error("expected error on start fail")
		}
		if err := s.Stop(context.Background()); err == nil {
			t.Error("expected error on stop fail")
		}
	})

	t.Run("RegisterDefaultServices", func(t *testing.T) {
		reg := NewRegistry()
		RegisterDefaultServices(reg)
		if reg.Get("systemd") == nil {
			t.Error("systemd was not registered via RegisterDefaultServices")
		}
	})
}
