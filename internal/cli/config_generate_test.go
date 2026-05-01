package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2"
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

func TestSurveyValidator(t *testing.T) {
	valFunc := func(v string) error {
		if v == "fail" {
			return errors.New("validation failed")
		}
		return nil
	}

	validator := surveyValidator(valFunc)

	if err := validator("fail"); err == nil || err.Error() != "validation failed" {
		t.Errorf("expected validation to fail, got %v", err)
	}
	if err := validator("success"); err != nil {
		t.Errorf("expected validation to succeed, got %v", err)
	}
	if err := validator(123); err != nil {
		t.Errorf("expected non-string to return nil, got %v", err)
	}
}

func buildMockSurveyAskOne(triggerErrorOn string) func(survey.Prompt, interface{}, ...survey.AskOpt) error {
	return func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		var msg string
		switch pt := p.(type) {
		case *survey.Input:
			msg = pt.Message
		case *survey.Select:
			msg = pt.Message
		}

		if triggerErrorOn != "" && msg == triggerErrorOn {
			return errors.New("simulated survey error")
		}

		switch pt := p.(type) {
		case *survey.Input:
			switch pt.Message {
			case "Name (how Home Assistant will refer to your machine):":
				*(response.(*string)) = "my-host"
			case "Enter server address:":
				*(response.(*string)) = "192.168.1.100"
			case "WOL Broadcast Address:":
				*(response.(*string)) = "192.168.1.255"
			case "Wake-on-LAN Port (leave blank for default):":
				*(response.(*string)) = "" // test fallback
			case "Bootloader Config Path:":
				*(response.(*string)) = "/boot/grub/grub.cfg"
			case "Home Assistant URL:":
				*(response.(*string)) = "http://hass.local:8123"
			case "Home Assistant Webhook ID:":
				*(response.(*string)) = "webhook123"
			}
		case *survey.Select:
			switch pt.Message {
			case "Server address for ping checks (Warning: If you choose an IP, it must be static):":
				*(response.(*string)) = "detected-host"
			case "Home Assistant Entity Type (buttons cannot track on/off states, switches can):":
				*(response.(*string)) = "switch"
			case "Select Physical WOL Interface":
				*(response.(*string)) = "eth0"
			case "Multiple WOL Subnet/Broadcast Addresses were discovered. Please select one:":
				*(response.(*string)) = "192.168.1.255"
			case "Bootloader:":
				*(response.(*string)) = "grub"
			case "Init System:":
				*(response.(*string)) = "systemd"
			}
		}
		return nil
	}
}

func TestGenerateConfigSurvey_Success(t *testing.T) {
	oldSurveyAskOne := surveyAskOne
	oldDiscoverHomeAssistant := discoverHomeAssistant
	oldDetectSystemHostname := detectSystemHostname
	oldGetSystemInterfaces := getSystemInterfaces
	defer func() {
		surveyAskOne = oldSurveyAskOne
		discoverHomeAssistant = oldDiscoverHomeAssistant
		detectSystemHostname = oldDetectSystemHostname
		getSystemInterfaces = oldGetSystemInterfaces
	}()

	surveyAskOne = buildMockSurveyAskOne("")

	discoverHomeAssistant = func(ctx context.Context) (string, error) { return "http://hass.local:8123", nil }
	detectSystemHostname = func() (string, error) { return "detected-host", nil }
	getSystemInterfaces = func() ([]system.InterfaceInfo, error) {
		return []system.InterfaceInfo{
			{
				Label:       "eth0",
				MAC:         "00:11:22:33:44:55",
				IPs:         []string{"192.168.1.100", "10.0.0.100"},
				IPBroadcast: map[string]string{"192.168.1.100": "192.168.1.255", "10.0.0.100": "10.0.0.255"},
			},
		}, nil
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

func TestGenerateConfigSurvey_AskOneErrors(t *testing.T) {
	oldSurveyAskOne := surveyAskOne
	oldDiscoverHomeAssistant := discoverHomeAssistant
	oldDetectSystemHostname := detectSystemHostname
	oldGetSystemInterfaces := getSystemInterfaces
	defer func() {
		surveyAskOne = oldSurveyAskOne
		discoverHomeAssistant = oldDiscoverHomeAssistant
		detectSystemHostname = oldDetectSystemHostname
		getSystemInterfaces = oldGetSystemInterfaces
	}()

	discoverHomeAssistant = func(ctx context.Context) (string, error) { return "http://hass.local:8123", nil }
	detectSystemHostname = func() (string, error) { return "detected-host", nil }
	getSystemInterfaces = func() ([]system.InterfaceInfo, error) {
		return []system.InterfaceInfo{{
			Label: "eth0",
			MAC:   "00:11:22:33:44:55",
			IPs:   []string{"192.168.1.100"},
		}}, nil
	}

	deps := setupSurveyDeps()
	errorSteps := []string{
		"Home Assistant Entity Type (buttons cannot track on/off states, switches can):",
		"Name (how Home Assistant will refer to your machine):",
		"Select Physical WOL Interface",
		"Server address for ping checks (Warning: If you choose an IP, it must be static):",
		"WOL Broadcast Address:",
		"Wake-on-LAN Port (leave blank for default):",
		"Bootloader:",
		"Bootloader Config Path:",
		"Init System:",
		"Home Assistant URL:",
		"Home Assistant Webhook ID:",
	}

	for _, step := range errorSteps {
		t.Run("Error at "+step, func(t *testing.T) {
			surveyAskOne = buildMockSurveyAskOne(step)
			_, err := generateConfigInteractive(context.Background(), deps)
			if err == nil || err.Error() != "simulated survey error" {
				t.Fatalf("expected simulated survey error at step %q, got %v", step, err)
			}
		})
	}

	t.Run("Multiple Subnet Selection Error", func(t *testing.T) {
		surveyAskOne = buildMockSurveyAskOne("Multiple WOL Subnet/Broadcast Addresses were discovered. Please select one:")
		getSystemInterfaces = func() ([]system.InterfaceInfo, error) {
			return []system.InterfaceInfo{{
				Label: "eth0", MAC: "00:11:22:33:44:55",
				IPBroadcast: map[string]string{"192.168.1.100": "192.168.1.255", "10.0.0.100": "10.0.0.255"},
			}}, nil
		}
		_, err := generateConfigInteractive(context.Background(), deps)
		if err == nil || err.Error() != "simulated survey error" {
			t.Errorf("expected simulated survey error, got %v", err)
		}
	})
}

func TestGenerateConfigSurvey_OptErrors(t *testing.T) {
	t.Run("Invalid MAC Address", func(t *testing.T) {
		oldSurveyAskOne := surveyAskOne
		oldDetectSystemHostname := detectSystemHostname
		oldGetSystemInterfaces := getSystemInterfaces
		surveyAskOne = buildMockSurveyAskOne("")
		defer func() {
			surveyAskOne = oldSurveyAskOne
			detectSystemHostname = oldDetectSystemHostname
			getSystemInterfaces = oldGetSystemInterfaces
		}()

		detectSystemHostname = func() (string, error) { return "host", nil }
		getSystemInterfaces = func() ([]system.InterfaceInfo, error) {
			return []system.InterfaceInfo{{Label: "eth0", MAC: "invalid-mac"}}, nil
		}

		deps := setupSurveyDeps()
		_, err := generateConfigInteractive(context.Background(), deps)
		if err == nil {
			t.Errorf("expected mac validation error, got nil")
		}
	})
}
