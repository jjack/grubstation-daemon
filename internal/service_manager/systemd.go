//go:build linux

package service_manager

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

const systemdName = "systemd"

var systemdDir = "/run/systemd/system"

var (
	systemdServicePath = "/etc/systemd/system/grubstation.service"
	osExecutable       = os.Executable
	osWriteFile        = os.WriteFile
	execCommand        = exec.CommandContext
)

//go:embed templates/grubstation.service.tmpl
var systemdTemplate string

type Systemd struct{}

func NewSystemd() Manager {
	return &Systemd{}
}

// RegisterDefaultServices registers systemd as the service manager on Linux.
func RegisterDefaultServices(r *Registry) {
	r.Register(systemdName, NewSystemd)
}

func (s *Systemd) Name() string {
	return systemdName
}

func (s *Systemd) IsActive(ctx context.Context) bool {
	fi, err := os.Stat(systemdDir)
	return err == nil && fi.IsDir()
}

func (s *Systemd) Install(ctx context.Context, configPath string) error {
	absConfig, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute config path: %w", err)
	}

	execPath, err := osExecutable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	tmpl, err := template.New("systemd").Parse(systemdTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse systemd template: %w", err)
	}

	data := struct {
		ExecPath   string
		ConfigPath string
	}{
		ExecPath:   execPath,
		ConfigPath: absConfig,
	}

	var content strings.Builder
	if err := tmpl.Execute(&content, data); err != nil {
		return fmt.Errorf("failed to execute systemd template: %w", err)
	}

	if err := osWriteFile(systemdServicePath, []byte(content.String()), 0o644); err != nil {
		return fmt.Errorf("failed to write systemd service file (are you running as root?): %w", err)
	}

	if out, err := execCommand(ctx, "systemctl", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %s", string(out))
	}

	if out, err := execCommand(ctx, "systemctl", "enable", "grubstation.service").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to enable systemd service: %s", string(out))
	}

	return nil
}

func (s *Systemd) Uninstall(ctx context.Context) error {
	_ = execCommand(ctx, "systemctl", "stop", "grubstation.service").Run()
	_ = execCommand(ctx, "systemctl", "disable", "grubstation.service").Run()

	if err := os.Remove(systemdServicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove systemd service file: %w", err)
	}

	if out, err := execCommand(ctx, "systemctl", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %s", string(out))
	}

	return nil
}

func (s *Systemd) Start(ctx context.Context) error {
	if out, err := execCommand(ctx, "systemctl", "start", "grubstation.service").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start systemd service: %s", string(out))
	}
	return nil
}

func (s *Systemd) Stop(ctx context.Context) error {
	if out, err := execCommand(ctx, "systemctl", "stop", "grubstation.service").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stop systemd service: %s", string(out))
	}
	return nil
}
