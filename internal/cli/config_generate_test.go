package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
	"github.com/jjack/remote-boot-agent/internal/initsystem"
	"github.com/jjack/remote-boot-agent/internal/system"
)

type mockGenInitSystem struct{ active bool }

func (m *mockGenInitSystem) Name() string                                         { return "mock-init" }
func (m *mockGenInitSystem) IsActive(ctx context.Context) bool                    { return m.active }
func (m *mockGenInitSystem) Install(ctx context.Context, configPath string) error { return nil }

type mockDiscoverFailBootloader struct{}

func (m *mockDiscoverFailBootloader) Name() string                      { return "discover-fail" }
func (m *mockDiscoverFailBootloader) IsActive(ctx context.Context) bool { return true }
func (m *mockDiscoverFailBootloader) GetBootOptions(ctx context.Context, cfg bootloader.Config) ([]string, error) {
	return nil, nil
}

func (m *mockDiscoverFailBootloader) Install(ctx context.Context, macAddress, haURL string) error {
	return nil
}

func (m *mockDiscoverFailBootloader) DiscoverConfigPath(ctx context.Context) (string, error) {
	return "", errors.New("discover fail")
}

type mockInactiveBootloader struct{}

func (m *mockInactiveBootloader) Name() string                      { return "inactive-bl" }
func (m *mockInactiveBootloader) IsActive(ctx context.Context) bool { return false }
func (m *mockInactiveBootloader) GetBootOptions(ctx context.Context, cfg bootloader.Config) ([]string, error) {
	return nil, nil
}

func (m *mockInactiveBootloader) Install(ctx context.Context, macAddress, haURL string) error {
	return nil
}

func (m *mockInactiveBootloader) DiscoverConfigPath(ctx context.Context) (string, error) {
	return "", nil
}

func TestGenerateConfigCmd_Execute(t *testing.T) {
	oldDiscover := discoverHomeAssistant
	oldDetectHostname := detectSystemHostname
	oldGetInterfaces := getSystemInterfaces
	oldRunForm := runGenerateSurvey
	oldSave := saveConfigFile

	defer func() {
		discoverHomeAssistant = oldDiscover
		detectSystemHostname = oldDetectHostname
		getSystemInterfaces = oldGetInterfaces
		runGenerateSurvey = oldRunForm
		saveConfigFile = oldSave
	}()

	tests := []struct {
		name        string
		setupMocks  func(*CommandDeps)
		wantErr     bool
		errContains string
	}{
		{
			name: "Happy Path",
			setupMocks: func(deps *CommandDeps) {
				discoverHomeAssistant = func() (string, error) { return "http://hass.local", nil }
				detectSystemHostname = func() (string, error) { return "test-host", nil }
				getSystemInterfaces = func() ([]system.InterfaceInfo, error) {
					return []system.InterfaceInfo{{Label: "eth0", Value: "00:11:22:33:44:55"}}, nil
				}
				runGenerateSurvey = func(opts GenerateSurveyOptions) (*config.Config, error) {
					if _, err := opts.DetectHostname(); err != nil {
						return nil, err
					}
					if _, err := opts.GetInterfaces(); err != nil {
						return nil, err
					}
					return &config.Config{}, nil
				}
				saveConfigFile = func(cfg *config.Config, path string) error { return nil }
			},
			wantErr: false,
		},
		{
			name: "Hostname Error",
			setupMocks: func(deps *CommandDeps) {
				detectSystemHostname = func() (string, error) { return "", errors.New("hostname fail") }
				runGenerateSurvey = func(opts GenerateSurveyOptions) (*config.Config, error) {
					if _, err := opts.DetectHostname(); err != nil {
						return nil, err
					}
					return &config.Config{}, nil
				}
			},
			wantErr:     true,
			errContains: "hostname fail",
		},
		{
			name: "Interfaces Error",
			setupMocks: func(deps *CommandDeps) {
				detectSystemHostname = func() (string, error) { return "test-host", nil }
				getSystemInterfaces = func() ([]system.InterfaceInfo, error) { return nil, errors.New("iface fail") }
				runGenerateSurvey = func(opts GenerateSurveyOptions) (*config.Config, error) {
					if _, err := opts.DetectHostname(); err != nil {
						return nil, err
					}
					if _, err := opts.GetInterfaces(); err != nil {
						return nil, err
					}
					return &config.Config{}, nil
				}
			},
			wantErr:     true,
			errContains: "iface fail",
		},
		{
			name: "Bootloader Detection Error",
			setupMocks: func(deps *CommandDeps) {
				blReg := bootloader.NewRegistry()
				blReg.Register("inactive-bl", func() bootloader.Bootloader { return &mockInactiveBootloader{} })
				deps.BootloaderRegistry = blReg
			},
			wantErr:     true,
			errContains: "no supported bootloader detected. Please ensure you have one of the following installed: inactive-bl",
		},
		{
			name: "Init System Detection Error",
			setupMocks: func(deps *CommandDeps) {
				initReg := initsystem.NewRegistry()
				initReg.Register("mock-init", func() initsystem.InitSystem { return &mockGenInitSystem{active: false} })
				deps.InitRegistry = initReg
			},
			wantErr:     true,
			errContains: "no supported init system detected. Please ensure you have one of the following installed: mock-init",
		},
		{
			name: "Form Error",
			setupMocks: func(deps *CommandDeps) {
				detectSystemHostname = func() (string, error) { return "test-host", nil }
				getSystemInterfaces = func() ([]system.InterfaceInfo, error) { return []system.InterfaceInfo{}, nil }
				runGenerateSurvey = func(opts GenerateSurveyOptions) (*config.Config, error) {
					return nil, errors.New("form canceled")
				}
			},
			wantErr:     true,
			errContains: "form canceled",
		},
		{
			name: "DiscoverConfigPath Fails But Proceeds",
			setupMocks: func(deps *CommandDeps) {
				discoverHomeAssistant = func() (string, error) { return "http://hass.local", nil }
				detectSystemHostname = func() (string, error) { return "test-host", nil }
				getSystemInterfaces = func() ([]system.InterfaceInfo, error) {
					return []system.InterfaceInfo{{Label: "eth0", Value: "00:11:22:33:44:55"}}, nil
				}
				runGenerateSurvey = func(opts GenerateSurveyOptions) (*config.Config, error) {
					return &config.Config{}, nil
				}
				saveConfigFile = func(cfg *config.Config, path string) error { return nil }

				blReg := bootloader.NewRegistry()
				blReg.Register("discover-fail", func() bootloader.Bootloader { return &mockDiscoverFailBootloader{} })
				deps.BootloaderRegistry = blReg
			},
			wantErr: false,
		},
		{
			name: "Save Config Error",
			setupMocks: func(deps *CommandDeps) {
				detectSystemHostname = func() (string, error) { return "test-host", nil }
				getSystemInterfaces = func() ([]system.InterfaceInfo, error) { return []system.InterfaceInfo{}, nil }
				runGenerateSurvey = func(opts GenerateSurveyOptions) (*config.Config, error) { return &config.Config{}, nil }
				saveConfigFile = func(cfg *config.Config, path string) error { return errors.New("save fail") }
			},
			wantErr:     true,
			errContains: "save fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blReg := bootloader.NewRegistry()
			blReg.Register("example", func() bootloader.Bootloader { return &mockListBootloader{} })
			initReg := initsystem.NewRegistry()
			initReg.Register("mock", func() initsystem.InitSystem { return &mockGenInitSystem{active: true} })

			deps := &CommandDeps{BootloaderRegistry: blReg, InitRegistry: initReg}
			tt.setupMocks(deps)
			cmd := NewConfigGenerateCmd(deps)
			cmd.SetArgs([]string{}) // prevent picking up real os.Args

			var b bytes.Buffer
			cmd.SetOut(&b)
			cmd.SetErr(&b)

			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("expected error: %v, got: %v", tt.wantErr, err)
			}
			if err != nil && tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("expected error to contain '%s', got '%v'", tt.errContains, err)
			}
		})
	}
}
