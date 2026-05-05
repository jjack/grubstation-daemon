package cli

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
	"github.com/jjack/remote-boot-agent/internal/initsystem"
)

type mockInstallBootloader struct {
	installErr error
	mac        string
	url        string
	webhook    string
}

func (m *mockInstallBootloader) Name() string                      { return "mock-bl" }
func (m *mockInstallBootloader) IsActive(ctx context.Context) bool { return true }
func (m *mockInstallBootloader) GetBootOptions(ctx context.Context, cfg bootloader.Config) ([]string, error) {
	return nil, nil
}

func (m *mockInstallBootloader) Setup(ctx context.Context, macAddress, haURL, webhookID string) error {
	m.mac = macAddress
	m.url = haURL
	m.webhook = webhookID
	return m.installErr
}

func (m *mockInstallBootloader) DiscoverConfigPath(ctx context.Context) (string, error) {
	return "", nil
}

type mockInstallInitSystem struct {
	installErr error
	configPath string
}

func (m *mockInstallInitSystem) Name() string                      { return "mock-init" }
func (m *mockInstallInitSystem) IsActive(ctx context.Context) bool { return true }
func (m *mockInstallInitSystem) Setup(ctx context.Context, configPath string) error {
	m.configPath = configPath
	return m.installErr
}

func TestSetupCmd_Success(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Host: config.HostConfig{
			MACAddress: "aa:bb:cc:dd:ee:ff",
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       "http://ha.local",
			WebhookID: "test-webhook",
		},
		Bootloader: config.BootloaderConfig{Name: "mock-bl"},
		InitSystem: config.InitSystemConfig{Name: "mock-init"},
	}

	blMock := &mockInstallBootloader{}
	blReg := bootloader.NewRegistry()
	blReg.Register("mock-bl", func() bootloader.Bootloader { return blMock })

	initMock := &mockInstallInitSystem{}
	initReg := initsystem.NewRegistry()
	initReg.Register("mock-init", func() initsystem.InitSystem { return initMock })

	deps := &CommandDeps{
		Config:             cfg,
		BootloaderRegistry: blReg,
		InitRegistry:       initReg,
	}

	cmd := NewSetupCmd(deps)
	cmd.Flags().String("config", "test-config.yaml", "")

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if blMock.mac != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("expected mac aa:bb:cc:dd:ee:ff, got %s", blMock.mac)
	}
	if blMock.url != "http://ha.local" {
		t.Errorf("expected url http://ha.local, got %s", blMock.url)
	}
	if blMock.webhook != "test-webhook" {
		t.Errorf("expected webhook test-webhook, got %s", blMock.webhook)
	}
	if !strings.HasSuffix(initMock.configPath, "test-config.yaml") {
		t.Errorf("expected config path to end with test-config.yaml, got %s", initMock.configPath)
	}
	if !filepath.IsAbs(initMock.configPath) {
		t.Errorf("expected absolute config path, got %s", initMock.configPath)
	}
	if !strings.Contains(out.String(), "Installation completed successfully") {
		t.Errorf("expected success message, got %s", out.String())
	}
}

func TestSetupCmd_BootloaderError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Bootloader: config.BootloaderConfig{Name: "mock-bl"},
		InitSystem: config.InitSystemConfig{Name: "mock-init"},
	}

	blReg := bootloader.NewRegistry()
	blReg.Register("mock-bl", func() bootloader.Bootloader { return &mockInstallBootloader{installErr: errors.New("fail")} })
	initReg := initsystem.NewRegistry()
	initReg.Register("mock-init", func() initsystem.InitSystem { return &mockInstallInitSystem{} })

	deps := &CommandDeps{Config: cfg, BootloaderRegistry: blReg, InitRegistry: initReg}
	cmd := NewSetupCmd(deps)
	cmd.Flags().String("config", "config.yaml", "")

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "failed to install bootloader") {
		t.Fatalf("expected bootloader install error, got %v", err)
	}
}

func TestSetupCmd_InitSystemError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Bootloader: config.BootloaderConfig{Name: "mock-bl"},
		InitSystem: config.InitSystemConfig{Name: "mock-init"},
	}

	blReg := bootloader.NewRegistry()
	blReg.Register("mock-bl", func() bootloader.Bootloader { return &mockInstallBootloader{} })
	initReg := initsystem.NewRegistry()
	initReg.Register("mock-init", func() initsystem.InitSystem { return &mockInstallInitSystem{installErr: errors.New("fail")} })

	deps := &CommandDeps{Config: cfg, BootloaderRegistry: blReg, InitRegistry: initReg}
	cmd := NewSetupCmd(deps)
	cmd.Flags().String("config", "config.yaml", "")

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "failed to install init system") {
		t.Fatalf("expected init system install error, got %v", err)
	}
}

func TestSetupCmd_MissingConfigFlag(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Bootloader: config.BootloaderConfig{Name: "mock-bl"},
		InitSystem: config.InitSystemConfig{Name: "mock-init"},
	}

	blReg := bootloader.NewRegistry()
	blReg.Register("mock-bl", func() bootloader.Bootloader { return &mockInstallBootloader{} })
	initReg := initsystem.NewRegistry()
	initReg.Register("mock-init", func() initsystem.InitSystem { return &mockInstallInitSystem{} })

	deps := &CommandDeps{Config: cfg, BootloaderRegistry: blReg, InitRegistry: initReg}
	cmd := NewSetupCmd(deps) // Missing binding the "config" flag locally

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "flag accessed but not defined") {
		t.Fatalf("expected flag missing error, got %v", err)
	}
}
