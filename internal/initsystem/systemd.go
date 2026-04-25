package initsystem

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
	systemdServicePath = "/etc/systemd/system/remote-boot-agent.service"
	osExecutable       = os.Executable
	osWriteFile        = os.WriteFile
	execCommand        = exec.CommandContext
)

//go:embed templates/remote-boot-agent.service.tmpl
var systemdTemplate string

type Systemd struct{}

func NewSystemd() InitSystem {
	return &Systemd{}
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

	if out, err := execCommand(ctx, "systemctl", "enable", "remote-boot-agent.service").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to enable systemd service: %s", string(out))
	}

	return nil
}
