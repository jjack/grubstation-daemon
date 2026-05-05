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
		name           string
		setupMocks     func(*CommandDeps)
		wantErr        bool
		errContains    string
		outputContains string
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
			name: "Happy Path Custom Config",
			setupMocks: func(deps *CommandDeps) {
				runGenerateSurvey = func(ctx context.Context, deps *CommandDeps) (*config.Config, error) {
					return &config.Config{
						Host: config.HostConfig{
							Name:             "test",
							Address:          "1.1.1.1",
							MACAddress:       "00:11:22:33:44:55",
							BroadcastAddress: "1.2.3.4",
							BroadcastPort:    7,
						},
					}, nil
				}
				saveConfigFile = func(cfg *config.Config, path string) error { return nil }
			},
			wantErr:        false,
			outputContains: "broadcast_address: 1.2.3.4",
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
			if tt.outputContains != "" && !strings.Contains(b.String(), tt.outputContains) {
				t.Errorf("expected output to contain '%s', got '%s'", tt.outputContains, b.String())
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
	oldRunHostInfoForm := runHostInfoForm
	oldRunWOLForm := runWOLForm
	oldRunBootloaderForm := runBootloaderForm
	oldRunInitSystemForm := runInitSystemForm
	oldRunHAForm := runHAForm
	oldDiscoverHomeAssistant := discoverHomeAssistant
	oldDetectSystemHostname := detectSystemHostname
	oldGetWOLInterfaces := getWOLInterfaces
	oldGetIPv4Info := getIPv4Info
	oldGetFQDN := getFQDN
	defer func() {
		runHostInfoForm = oldRunHostInfoForm
		runWOLForm = oldRunWOLForm
		runBootloaderForm = oldRunBootloaderForm
		runInitSystemForm = oldRunInitSystemForm
		runHAForm = oldRunHAForm
		discoverHomeAssistant = oldDiscoverHomeAssistant
		detectSystemHostname = oldDetectSystemHostname
		getWOLInterfaces = oldGetWOLInterfaces
		getIPv4Info = oldGetIPv4Info
		getFQDN = oldGetFQDN
	}()

	runInitSystemForm = func(io []string) (initSystemResults, error) {
		return initSystemResults{Name: "systemd"}, nil
	}
	runBootloaderForm = func(bo []string, d *CommandDeps, c context.Context) (bootloaderResults, error) {
		return bootloaderResults{Name: "grub", ConfigPath: "/boot/grub/grub.cfg"}, nil
	}
	runHostInfoForm = func(io []huh.Option[string], im map[string]net.Interface, h string) (hostInfoResults, []huh.Option[string], error) {
		bOpts := []huh.Option[string]{huh.NewOption("Subnet", "192.168.1.255")}
		return hostInfoResults{Name: "test-name", IfaceName: "eth0", MACAddress: "00:11:22:33:44:55", HostAddress: "192.168.1.100"}, bOpts, nil
	}
	runWOLForm = func(bo []huh.Option[string]) (wolResults, error) {
		return wolResults{Broadcast: "192.168.1.255", WOLPort: "9"}, nil
	}
	runHAForm = func(u string) (haResults, error) {
		return haResults{URL: "http://hass.local:8123", WebhookID: "webhook123"}, nil
	}

	discoverHomeAssistant = func(ctx context.Context) (string, error) { return "http://hass.local:8123", nil }
	detectSystemHostname = func() (string, error) { return "detected-host", nil }

	getWOLInterfaces = func() ([]net.Interface, error) {
		mac, _ := net.ParseMAC("00:11:22:33:44:55")
		return []net.Interface{{Name: "eth0", HardwareAddr: mac}}, nil
	}
	getFQDN = func(hostname string) string { return "detected-host.local" }
	getIPv4Info = func(inf net.Interface) ([]string, map[string]string) {
		return []string{"192.168.1.100", "10.0.0.100"}, map[string]string{"192.168.1.100": "192.168.1.255", "10.0.0.100": "10.0.0.255"}
	}

	deps := setupSurveyDeps()
	cfg, err := generateConfigInteractive(context.Background(), deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Host.Name != "test-name" {
		t.Errorf("expected name test-name, got %s", cfg.Host.Name)
	}
	if cfg.Host.Address != "192.168.1.100" {
		t.Errorf("expected address 192.168.1.100, got %s", cfg.Host.Address)
	}
	if cfg.Host.BroadcastAddress != "192.168.1.255" {
		t.Errorf("expected BroadcastAddress 192.168.1.255, got %s", cfg.Host.BroadcastAddress)
	}
	if cfg.Host.BroadcastPort != 9 {
		t.Errorf("expected BroadcastPort 9 (fallback), got %d", cfg.Host.BroadcastPort)
	}
	if cfg.HomeAssistant.URL != "http://hass.local:8123" {
		t.Errorf("expected URL http://hass.local:8123, got %s", cfg.HomeAssistant.URL)
	}
}

func TestGenerateConfigSurvey_FormErrors(t *testing.T) {
	oldRunHostInfoForm := runHostInfoForm
	oldRunWOLForm := runWOLForm
	oldRunBootloaderForm := runBootloaderForm
	oldRunInitSystemForm := runInitSystemForm
	oldRunHAForm := runHAForm
	oldDiscoverHomeAssistant := discoverHomeAssistant
	oldDetectSystemHostname := detectSystemHostname
	oldGetWOLInterfaces := getWOLInterfaces
	oldGetIPv4Info := getIPv4Info
	oldGetFQDN := getFQDN
	defer func() {
		runHostInfoForm = oldRunHostInfoForm
		runWOLForm = oldRunWOLForm
		runBootloaderForm = oldRunBootloaderForm
		runInitSystemForm = oldRunInitSystemForm
		runHAForm = oldRunHAForm
		discoverHomeAssistant = oldDiscoverHomeAssistant
		detectSystemHostname = oldDetectSystemHostname
		getWOLInterfaces = oldGetWOLInterfaces
		getIPv4Info = oldGetIPv4Info
		getFQDN = oldGetFQDN
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
	getFQDN = func(hostname string) string { return "detected-host.local" }

	deps := setupSurveyDeps()

	resetMocks := func() {
		runInitSystemForm = func(io []string) (initSystemResults, error) { return initSystemResults{Name: "systemd"}, nil }
		runBootloaderForm = func(bo []string, d *CommandDeps, c context.Context) (bootloaderResults, error) {
			return bootloaderResults{Name: "grub", ConfigPath: "/boot/grub/grub.cfg"}, nil
		}
		runHostInfoForm = func(io []huh.Option[string], im map[string]net.Interface, h string) (hostInfoResults, []huh.Option[string], error) {
			return hostInfoResults{Name: "test-name", IfaceName: "eth0", MACAddress: "00:11:22:33:44:55", HostAddress: "192.168.1.100"}, []huh.Option[string]{huh.NewOption("test", "test")}, nil
		}
		runWOLForm = func(bo []huh.Option[string]) (wolResults, error) {
			return wolResults{Broadcast: "192.168.1.255", WOLPort: "9"}, nil
		}
		runHAForm = func(u string) (haResults, error) {
			return haResults{URL: "http://hass.local:8123", WebhookID: "webhook123"}, nil
		}
	}
	resetMocks()

	t.Run("Init System Form Error", func(t *testing.T) {
		runInitSystemForm = func(io []string) (initSystemResults, error) {
			return initSystemResults{}, errors.New("simulated init error")
		}
		_, err := generateConfigInteractive(context.Background(), deps)
		if err == nil || err.Error() != "simulated init error" {
			t.Fatalf("expected simulated init error, got %v", err)
		}
		resetMocks()
	})

	t.Run("Bootloader Form Error", func(t *testing.T) {
		runBootloaderForm = func(bo []string, d *CommandDeps, c context.Context) (bootloaderResults, error) {
			return bootloaderResults{}, errors.New("simulated bl error")
		}
		_, err := generateConfigInteractive(context.Background(), deps)
		if err == nil || err.Error() != "simulated bl error" {
			t.Fatalf("expected simulated bl error, got %v", err)
		}
		resetMocks()
	})

	t.Run("Host Info Form Error", func(t *testing.T) {
		runHostInfoForm = func(io []huh.Option[string], im map[string]net.Interface, h string) (hostInfoResults, []huh.Option[string], error) {
			return hostInfoResults{}, nil, errors.New("simulated host info error")
		}
		_, err := generateConfigInteractive(context.Background(), deps)
		if err == nil || err.Error() != "simulated host info error" {
			t.Fatalf("expected simulated host info error, got %v", err)
		}
		resetMocks()
	})

	t.Run("WOL Form Error", func(t *testing.T) {
		runWOLForm = func(bo []huh.Option[string]) (wolResults, error) {
			return wolResults{}, errors.New("simulated wol error")
		}
		_, err := generateConfigInteractive(context.Background(), deps)
		if err == nil || err.Error() != "simulated wol error" {
			t.Fatalf("expected simulated wol error, got %v", err)
		}
		resetMocks()
	})

	t.Run("HA Form Error", func(t *testing.T) {
		runHAForm = func(u string) (haResults, error) { return haResults{}, errors.New("simulated ha error") }
		_, err := generateConfigInteractive(context.Background(), deps)
		if err == nil || err.Error() != "simulated ha error" {
			t.Fatalf("expected simulated ha error, got %v", err)
		}
		resetMocks()
	})
}

func TestGenerateConfigSurvey_OptErrors(t *testing.T) {
	t.Run("Invalid MAC Address", func(t *testing.T) {
		oldRunHostInfoForm := runHostInfoForm
		oldDetectSystemHostname := detectSystemHostname
		oldGetWOLInterfaces := getWOLInterfaces

		runHostInfoForm = func(io []huh.Option[string], im map[string]net.Interface, h string) (hostInfoResults, []huh.Option[string], error) {
			return hostInfoResults{Name: "test", IfaceName: "eth0", MACAddress: "", HostAddress: "192.168.1.100"}, []huh.Option[string]{}, nil
		}
		defer func() {
			runHostInfoForm = oldRunHostInfoForm
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

func TestBuildIfaceOptions(t *testing.T) {
	oldGetIPv4Info := getIPv4Info
	defer func() { getIPv4Info = oldGetIPv4Info }()
	getIPv4Info = func(inf net.Interface) ([]string, map[string]string) {
		return []string{"192.168.1.50"}, nil
	}

	mac, _ := net.ParseMAC("00:11:22:33:44:55")
	ifaces := []net.Interface{
		{Name: "eth0", HardwareAddr: mac},
	}

	opts, m := buildIfaceOptions(ifaces)
	if len(opts) != 1 {
		t.Fatalf("expected 1 option, got %d", len(opts))
	}
	if len(m) != 1 {
		t.Fatalf("expected map of len 1, got %d", len(m))
	}

	expectedLabel := "eth0 (00:11:22:33:44:55) [192.168.1.50]"
	if opts[0].Key != expectedLabel {
		t.Errorf("expected label %s, got %s", expectedLabel, opts[0].Key)
	}
}

func TestBuildHostOptions(t *testing.T) {
	opts := buildHostOptions("my-host", "my-host.local", []string{"192.168.1.50"})

	if len(opts) != 4 {
		t.Fatalf("expected 4 options, got %d", len(opts))
	}
	if opts[0].Value != "my-host" {
		t.Errorf("expected option 0 to be my-host")
	}
	if opts[1].Value != "my-host.local" {
		t.Errorf("expected option 1 to be my-host.local")
	}
	if opts[2].Value != "192.168.1.50" {
		t.Errorf("expected option 2 to be 192.168.1.50")
	}
	if opts[3].Value != OptionCustomHost {
		t.Errorf("expected option 3 to be Custom")
	}

	// Test without FQDN
	optsNoFqdn := buildHostOptions("my-host", "my-host", []string{"192.168.1.50"})
	if len(optsNoFqdn) != 3 {
		t.Fatalf("expected 3 options without fqdn, got %d", len(optsNoFqdn))
	}
}

func TestBuildBroadcastOptions(t *testing.T) {
	ips := []string{"192.168.1.50", "10.0.0.50"}
	broadcasts := map[string]string{
		"192.168.1.50": "192.168.1.255",
		"10.0.0.50":    "10.0.0.255",
	}

	opts := buildBroadcastOptions("192.168.1.50", ips, broadcasts)

	if len(opts) != 4 {
		t.Fatalf("expected 4 options, got %d", len(opts))
	}
	if opts[0].Value != config.DefaultBroadcastAddress {
		t.Errorf("expected DefaultBroadcastAddress, got %s", opts[0].Value)
	}
	if opts[1].Value != "192.168.1.255" {
		t.Errorf("expected subnet broadcast 192.168.1.255, got %s", opts[1].Value)
	}
	if opts[2].Value != "10.0.0.255" {
		t.Errorf("expected subnet broadcast 10.0.0.255, got %s", opts[2].Value)
	}
	if opts[3].Value != "custom" {
		t.Errorf("expected custom, got %s", opts[3].Value)
	}

	// Test deduplication
	ipsDup := []string{"192.168.1.50", "192.168.1.51"}
	broadcastsDup := map[string]string{
		"192.168.1.50": "192.168.1.255",
		"192.168.1.51": "192.168.1.255",
	}
	optsDup := buildBroadcastOptions("192.168.1.50", ipsDup, broadcastsDup)
	if len(optsDup) != 3 {
		t.Fatalf("expected 3 options due to dedup, got %d", len(optsDup))
	}

	// Test IPv6 filtering (if HostAddress is IPv4, it filters out IPv6 subnets)
	ipsMix := []string{"192.168.1.50", "fe80::1"}
	broadcastsMix := map[string]string{
		"192.168.1.50": "192.168.1.255",
		"fe80::1":      "fe80::ffff",
	}
	optsMix := buildBroadcastOptions("192.168.1.50", ipsMix, broadcastsMix)
	if len(optsMix) != 3 {
		t.Fatalf("expected 3 options due to ipv6 filtering, got %d", len(optsMix))
	}
}

func TestGenerateConfigSurvey_ContextCancelBeforeHA(t *testing.T) {
	oldRunHostInfoForm := runHostInfoForm
	oldRunWOLForm := runWOLForm
	oldRunBootloaderForm := runBootloaderForm
	oldRunInitSystemForm := runInitSystemForm
	oldRunHAForm := runHAForm
	oldDiscoverHomeAssistant := discoverHomeAssistant
	oldDetectSystemHostname := detectSystemHostname
	oldGetWOLInterfaces := getWOLInterfaces
	oldGetIPv4Info := getIPv4Info
	oldGetFQDN := getFQDN
	defer func() {
		runHostInfoForm = oldRunHostInfoForm
		runWOLForm = oldRunWOLForm
		runBootloaderForm = oldRunBootloaderForm
		runInitSystemForm = oldRunInitSystemForm
		runHAForm = oldRunHAForm
		discoverHomeAssistant = oldDiscoverHomeAssistant
		detectSystemHostname = oldDetectSystemHostname
		getWOLInterfaces = oldGetWOLInterfaces
		getIPv4Info = oldGetIPv4Info
		getFQDN = oldGetFQDN
	}()

	ctx, cancel := context.WithCancel(context.Background())

	discoverHomeAssistant = func(c context.Context) (string, error) { <-c.Done(); return "", c.Err() }
	detectSystemHostname = func() (string, error) { return "host", nil }
	getWOLInterfaces = func() ([]net.Interface, error) {
		return []net.Interface{{Name: "eth0", HardwareAddr: net.HardwareAddr{1, 2, 3, 4, 5, 6}}}, nil
	}
	getIPv4Info = func(net.Interface) ([]string, map[string]string) { return nil, nil }
	getFQDN = func(h string) string { return h }

	runInitSystemForm = func(io []string) (initSystemResults, error) { return initSystemResults{}, nil }
	runBootloaderForm = func(bo []string, d *CommandDeps, c context.Context) (bootloaderResults, error) {
		return bootloaderResults{}, nil
	}
	runHostInfoForm = func(io []huh.Option[string], im map[string]net.Interface, h string) (hostInfoResults, []huh.Option[string], error) {
		return hostInfoResults{MACAddress: "00:11:22:33:44:55"}, nil, nil
	}
	runWOLForm = func(bo []huh.Option[string]) (wolResults, error) { cancel(); return wolResults{WOLPort: "9"}, nil }
	runHAForm = func(u string) (haResults, error) { return haResults{}, nil }

	deps := setupSurveyDeps()
	if _, err := generateConfigInteractive(ctx, deps); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}
