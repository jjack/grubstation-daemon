package cli

import (
	"bytes"
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/jjack/remote-boot-agent/internal/bootloader"
	"github.com/jjack/remote-boot-agent/internal/config"
	"github.com/jjack/remote-boot-agent/internal/initsystem"
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

func (m *mockDiscoverFailBootloader) Install(ctx context.Context, macAddress, haURL, webhookID string) error {
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

func (m *mockInactiveBootloader) Install(ctx context.Context, macAddress, haURL, webhookID string) error {
	return nil
}

func (m *mockInactiveBootloader) DiscoverConfigPath(ctx context.Context) (string, error) {
	return "", nil
}

func TestGenerateConfigCmd_Execute(t *testing.T) {
	oldRunForm := runGenerateSurvey
	oldSave := saveConfigFile

	defer func() {
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
				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
					return &config.Config{}, nil
				}
				saveConfigFile = func(cfg *config.Config, path string) error { return nil }
			},
			wantErr: false,
		},
		{
			name: "Hostname Error",
			setupMocks: func(deps *CommandDeps) {
				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
					return nil, errors.New("hostname fail")
				}
			},
			wantErr:     true,
			errContains: "hostname fail",
		},
		{
			name: "Interfaces Error",
			setupMocks: func(deps *CommandDeps) {
				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
					return nil, errors.New("iface fail")
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
				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
					return nil, errors.New("form canceled")
				}
			},
			wantErr:     true,
			errContains: "form canceled",
		},
		{
			name: "DiscoverConfigPath Fails But Proceeds",
			setupMocks: func(deps *CommandDeps) {
				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
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
				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) { return &config.Config{}, nil }
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

type mockSurveyBootloader struct{}

func (m *mockSurveyBootloader) Name() string                      { return "grub" }
func (m *mockSurveyBootloader) IsActive(ctx context.Context) bool { return true }
func (m *mockSurveyBootloader) GetBootOptions(ctx context.Context, cfg bootloader.Config) ([]string, error) {
	return nil, nil
}

func (m *mockSurveyBootloader) Install(ctx context.Context, macAddress, haURL, webhookID string) error {
	return nil
}

func (m *mockSurveyBootloader) DiscoverConfigPath(ctx context.Context) (string, error) {
	return "/boot/grub/grub.cfg", nil
}

type mockSurveyInitSystem struct{}

func (m *mockSurveyInitSystem) Name() string                                         { return "systemd" }
func (m *mockSurveyInitSystem) IsActive(ctx context.Context) bool                    { return true }
func (m *mockSurveyInitSystem) Install(ctx context.Context, configPath string) error { return nil }

func setupSurveyDeps() *CommandDeps {
	blReg := bootloader.NewRegistry()
	blReg.Register("grub", func() bootloader.Bootloader { return &mockSurveyBootloader{} })

	initReg := initsystem.NewRegistry()
	initReg.Register("systemd", func() initsystem.InitSystem { return &mockSurveyInitSystem{} })

	return &CommandDeps{BootloaderRegistry: blReg, InitRegistry: initReg}
}

func TestGenerateConfigSurvey_Success(t *testing.T) {
	oldRunBasic := runBasicForm
	oldRunAdvanced := runAdvancedForm
	oldDiscoverHomeAssistant := discoverHomeAssistant
	oldDetectSystemHostname := detectSystemHostname
	oldGetWOLInterfaces := getWOLInterfaces
	oldGetIPv4Info := getIPv4Info
	defer func() {
		runBasicForm = oldRunBasic
		runAdvancedForm = oldRunAdvanced
		discoverHomeAssistant = oldDiscoverHomeAssistant
		detectSystemHostname = oldDetectSystemHostname
		getWOLInterfaces = oldGetWOLInterfaces
		getIPv4Info = oldGetIPv4Info
	}()

	runBasicForm = func(h, u string, io []huh.Option[string], bo, init []string) (basicFormResults, error) {
		return basicFormResults{
			EntityType: string(config.EntityTypeSwitch),
			Name:       "my-host",
			HAURL:      "http://hass.local:8123",
			HAWebhook:  "webhook123",
			Bootloader: "grub",
			InitSystem: "systemd",
			IfaceName:  "eth0",
		}, nil
	}
	runAdvancedForm = func(s bool, ho []huh.Option[string], dh, db, dbp string) (advancedFormResults, error) {
		return advancedFormResults{
			HostAddress:    "detected-host",
			Broadcast:      "192.168.1.255",
			WOLPort:        "9",
			BootloaderPath: "/boot/grub/grub.cfg",
		}, nil
	}

	discoverHomeAssistant = func(ctx context.Context) (string, error) { return "http://hass.local:8123", nil }
	detectSystemHostname = func() (string, error) { return "detected-host", nil }

	getWOLInterfaces = func() ([]net.Interface, error) {
		mac, _ := net.ParseMAC("00:11:22:33:44:55")
		return []net.Interface{{Name: "eth0", HardwareAddr: mac}}, nil
	}
	getIPv4Info = func(inf net.Interface) ([]string, map[string]string) {
		return []string{"192.168.1.100", "10.0.0.100"}, map[string]string{"192.168.1.100": "192.168.1.255", "10.0.0.100": "10.0.0.255"}
	}

	deps := setupSurveyDeps()
	cfg, err := generateConfigInteractive(context.Background(), deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Name != "my-host" {
		t.Errorf("expected name my-host, got %s", cfg.Server.Name)
	}
	if cfg.Server.Host != "detected-host" {
		t.Errorf("expected host detected-host, got %s", cfg.Server.Host)
	}
	if cfg.HomeAssistant.EntityType != config.EntityTypeSwitch {
		t.Errorf("expected entity type switch, got %s", cfg.HomeAssistant.EntityType)
	}
	if cfg.Server.BroadcastAddress != "192.168.1.255" {
		t.Errorf("expected BroadcastAddress 192.168.1.255, got %s", cfg.Server.BroadcastAddress)
	}
	if cfg.Server.BroadcastPort != 9 {
		t.Errorf("expected BroadcastPort 9 (fallback), got %d", cfg.Server.BroadcastPort)
	}
	if cfg.HomeAssistant.URL != "http://hass.local:8123" {
		t.Errorf("expected URL http://hass.local:8123, got %s", cfg.HomeAssistant.URL)
	}
}

func TestGenerateConfigSurvey_FormErrors(t *testing.T) {
	oldRunBasic := runBasicForm
	oldRunAdvanced := runAdvancedForm
	oldDiscoverHomeAssistant := discoverHomeAssistant
	oldDetectSystemHostname := detectSystemHostname
	oldGetWOLInterfaces := getWOLInterfaces
	oldGetIPv4Info := getIPv4Info
	defer func() {
		runBasicForm = oldRunBasic
		runAdvancedForm = oldRunAdvanced
		discoverHomeAssistant = oldDiscoverHomeAssistant
		detectSystemHostname = oldDetectSystemHostname
		getWOLInterfaces = oldGetWOLInterfaces
		getIPv4Info = oldGetIPv4Info
	}()

	discoverHomeAssistant = func(ctx context.Context) (string, error) { return "http://hass.local:8123", nil }
	detectSystemHostname = func() (string, error) { return "detected-host", nil }

	getWOLInterfaces = func() ([]net.Interface, error) {
		mac, _ := net.ParseMAC("00:11:22:33:44:55")
		return []net.Interface{{Name: "eth0", HardwareAddr: mac}}, nil
	}
	getIPv4Info = func(inf net.Interface) ([]string, map[string]string) {
		return []string{"192.168.1.100"}, map[string]string{"192.168.1.100": "192.168.1.255"}
	}

	deps := setupSurveyDeps()

	t.Run("Basic Form Error", func(t *testing.T) {
		runBasicForm = func(h, u string, io []huh.Option[string], bo, init []string) (basicFormResults, error) {
			return basicFormResults{}, errors.New("simulated basic error")
		}
		runAdvancedForm = func(s bool, ho []huh.Option[string], dh, db, dbp string) (advancedFormResults, error) {
			return advancedFormResults{}, nil
		}
		_, err := generateConfigInteractive(context.Background(), deps)
		if err == nil || err.Error() != "simulated basic error" {
			t.Fatalf("expected simulated basic error, got %v", err)
		}
	})

	t.Run("Advanced Form Error", func(t *testing.T) {
		runBasicForm = func(h, u string, io []huh.Option[string], bo, init []string) (basicFormResults, error) {
			return basicFormResults{IfaceName: "eth0"}, nil
		}
		runAdvancedForm = func(s bool, ho []huh.Option[string], dh, db, dbp string) (advancedFormResults, error) {
			return advancedFormResults{}, errors.New("simulated advanced error")
		}
		_, err := generateConfigInteractive(context.Background(), deps)
		if err == nil || err.Error() != "simulated advanced error" {
			t.Fatalf("expected simulated advanced error, got %v", err)
		}
	})
}

func TestGenerateConfigSurvey_OptErrors(t *testing.T) {
	t.Run("Invalid MAC Address", func(t *testing.T) {
		oldRunBasic := runBasicForm
		oldDetectSystemHostname := detectSystemHostname
		oldGetWOLInterfaces := getWOLInterfaces
		runBasicForm = func(h, u string, io []huh.Option[string], bo, init []string) (basicFormResults, error) {
			return basicFormResults{IfaceName: "eth0"}, nil
		}
		defer func() {
			runBasicForm = oldRunBasic
			detectSystemHostname = oldDetectSystemHostname
			getWOLInterfaces = oldGetWOLInterfaces
		}()

		detectSystemHostname = func() (string, error) { return "host", nil }
		getWOLInterfaces = func() ([]net.Interface, error) {
			return []net.Interface{{Name: "eth0", HardwareAddr: nil}}, nil
		}

		deps := setupSurveyDeps()
		_, err := generateConfigInteractive(context.Background(), deps)
		if err == nil {
			t.Errorf("expected mac validation error, got nil")
		}
	})
}
