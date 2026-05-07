package cli

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/jjack/remote-boot-agent/internal/config"
	"github.com/jjack/remote-boot-agent/internal/grub"
	"github.com/jjack/remote-boot-agent/internal/initsystem"
	"github.com/spf13/cobra"
)

type mockSystemResolver struct {
	discoverHomeAssistantFunc func(ctx context.Context) (string, error)
	detectSystemHostnameFunc  func() (string, error)
	getWOLInterfacesFunc      func() ([]net.Interface, error)
	getIPv4InfoFunc           func(inf net.Interface) ([]string, map[string]string)
	getFQDNFunc               func(hostname string) string
	saveConfigFunc            func(cfg *config.Config, path string) error
}

func (m *mockSystemResolver) DiscoverHomeAssistant(ctx context.Context) (string, error) {
	if m.discoverHomeAssistantFunc != nil {
		return m.discoverHomeAssistantFunc(ctx)
	}
	return "http://hass.local:8123", nil
}

func (m *mockSystemResolver) DetectSystemHostname() (string, error) {
	if m.detectSystemHostnameFunc != nil {
		return m.detectSystemHostnameFunc()
	}
	return "detected-host", nil
}

func (m *mockSystemResolver) GetWOLInterfaces() ([]net.Interface, error) {
	if m.getWOLInterfacesFunc != nil {
		return m.getWOLInterfacesFunc()
	}
	mac, _ := net.ParseMAC("00:11:22:33:44:55")
	return []net.Interface{{Name: "eth0", HardwareAddr: mac}}, nil
}

func (m *mockSystemResolver) GetIPv4Info(inf net.Interface) ([]string, map[string]string) {
	if m.getIPv4InfoFunc != nil {
		return m.getIPv4InfoFunc(inf)
	}
	return []string{"192.168.1.100"}, map[string]string{"192.168.1.100": "192.168.1.255"}
}

func (m *mockSystemResolver) GetFQDN(hostname string) string {
	if m.getFQDNFunc != nil {
		return m.getFQDNFunc(hostname)
	}
	return "detected-host.local"
}

func (m *mockSystemResolver) SaveConfig(cfg *config.Config, path string) error {
	if m.saveConfigFunc != nil {
		return m.saveConfigFunc(cfg, path)
	}
	return nil
}

type mockSurveyInitSystem struct{}

func (m *mockSurveyInitSystem) Name() string                                       { return "systemd" }
func (m *mockSurveyInitSystem) IsActive(ctx context.Context) bool                  { return true }
func (m *mockSurveyInitSystem) Setup(ctx context.Context, configPath string) error { return nil }

func setupSurveyDeps(t *testing.T) *CommandDeps {
	initReg := initsystem.NewRegistry()
	initReg.Register("systemd", func() initsystem.InitSystem { return &mockSurveyInitSystem{} })
	tempGrub := t.TempDir() + "/grub.cfg"
	_ = os.WriteFile(tempGrub, []byte(""), 0o644)
	return &CommandDeps{Grub: &grub.Grub{ConfigPath: tempGrub}, InitRegistry: initReg, SystemResolver: &mockSystemResolver{}}
}

func TestGenerateConfigSurvey_Success(t *testing.T) {
	oldRunHostInfoForm := runHostInfoForm
	oldRunWOLForm := runWOLForm
	oldRunInitSystemForm := runInitSystemForm
	oldRunHAForm := runHAForm
	defer func() {
		runHostInfoForm = oldRunHostInfoForm
		runWOLForm = oldRunWOLForm
		runInitSystemForm = oldRunInitSystemForm
		runHAForm = oldRunHAForm
	}()

	runInitSystemForm = func(io []string) (initSystemResults, error) {
		return initSystemResults{Name: "systemd"}, nil
	}
	runHostInfoForm = func(resolver SystemResolver, io []huh.Option[string], im map[string]net.Interface, h string) (hostInfoResults, []huh.Option[string], error) {
		bOpts := []huh.Option[string]{huh.NewOption("Subnet", "192.168.1.255")}
		return hostInfoResults{Name: "test-name", IfaceName: "eth0", MACAddress: "00:11:22:33:44:55", HostAddress: "192.168.1.100"}, bOpts, nil
	}
	runWOLForm = func(bo []huh.Option[string]) (wolResults, error) {
		return wolResults{Broadcast: "192.168.1.255", WOLPort: "9"}, nil
	}
	runHAForm = func(u string) (haResults, error) {
		return haResults{URL: "http://hass.local:8123", WebhookID: "webhook123"}, nil
	}

	deps := setupSurveyDeps(t)
	deps.SystemResolver = &mockSystemResolver{
		getIPv4InfoFunc: func(inf net.Interface) ([]string, map[string]string) {
			return []string{"192.168.1.100", "10.0.0.100"}, map[string]string{"192.168.1.100": "192.168.1.255", "10.0.0.100": "10.0.0.255"}
		},
	}

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
	oldRunInitSystemForm := runInitSystemForm
	oldRunHAForm := runHAForm
	defer func() {
		runHostInfoForm = oldRunHostInfoForm
		runWOLForm = oldRunWOLForm
		runInitSystemForm = oldRunInitSystemForm
		runHAForm = oldRunHAForm
	}()

	deps := setupSurveyDeps(t)

	resetMocks := func() {
		runInitSystemForm = func(io []string) (initSystemResults, error) { return initSystemResults{Name: "systemd"}, nil }
		runHostInfoForm = func(resolver SystemResolver, io []huh.Option[string], im map[string]net.Interface, h string) (hostInfoResults, []huh.Option[string], error) {
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

	t.Run("Host Info Form Error", func(t *testing.T) {
		runHostInfoForm = func(resolver SystemResolver, io []huh.Option[string], im map[string]net.Interface, h string) (hostInfoResults, []huh.Option[string], error) {
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

	t.Run("Detect System Hostname Error", func(t *testing.T) {
		d := setupSurveyDeps(t)
		d.SystemResolver = &mockSystemResolver{
			detectSystemHostnameFunc: func() (string, error) { return "", errors.New("simulated hostname error") },
		}
		_, err := generateConfigInteractive(context.Background(), d)
		if err == nil || err.Error() != "simulated hostname error" {
			t.Fatalf("expected simulated hostname error, got %v", err)
		}
	})

	t.Run("Get WOL Interfaces Error", func(t *testing.T) {
		d := setupSurveyDeps(t)
		d.SystemResolver = &mockSystemResolver{
			getWOLInterfacesFunc: func() ([]net.Interface, error) { return nil, errors.New("simulated wol interfaces error") },
		}
		_, err := generateConfigInteractive(context.Background(), d)
		if err == nil || err.Error() != "simulated wol interfaces error" {
			t.Fatalf("expected simulated wol interfaces error, got %v", err)
		}
	})
}

func TestGenerateConfigSurvey_OptErrors(t *testing.T) {
	t.Run("Invalid MAC Address", func(t *testing.T) {
		oldRunHostInfoForm := runHostInfoForm
		oldRunInitSystemForm := runInitSystemForm
		oldRunWOLForm := runWOLForm
		oldRunHAForm := runHAForm

		runInitSystemForm = func(io []string) (initSystemResults, error) { return initSystemResults{Name: "systemd"}, nil }
		runHostInfoForm = func(resolver SystemResolver, io []huh.Option[string], im map[string]net.Interface, h string) (hostInfoResults, []huh.Option[string], error) {
			return hostInfoResults{Name: "test", IfaceName: "eth0", MACAddress: "invalid-mac"}, []huh.Option[string]{}, nil
		}
		runWOLForm = func(bo []huh.Option[string]) (wolResults, error) {
			return wolResults{Broadcast: "192.168.1.255", WOLPort: "9"}, nil
		}
		runHAForm = func(u string) (haResults, error) {
			return haResults{URL: "http://hass.local:8123", WebhookID: "webhook123"}, nil
		}

		defer func() {
			runHostInfoForm = oldRunHostInfoForm
			runInitSystemForm = oldRunInitSystemForm
			runWOLForm = oldRunWOLForm
			runHAForm = oldRunHAForm
		}()

		deps := setupSurveyDeps(t)
		deps.SystemResolver = &mockSystemResolver{
			getWOLInterfacesFunc: func() ([]net.Interface, error) {
				return []net.Interface{{Name: "eth0", HardwareAddr: nil}}, nil
			},
		}

		_, err := generateConfigInteractive(context.Background(), deps)
		if err == nil {
			t.Errorf("expected mac validation error, got nil")
		}
	})
}

func TestBuildIfaceOptions(t *testing.T) {
	resolver := &mockSystemResolver{
		getIPv4InfoFunc: func(inf net.Interface) ([]string, map[string]string) {
			return []string{"192.168.1.50"}, nil
		},
	}
	mac, _ := net.ParseMAC("00:11:22:33:44:55")
	ifaces := []net.Interface{
		{Name: "eth0", HardwareAddr: mac},
	}

	opts, m := buildIfaceOptions(resolver, ifaces)
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
	oldRunInitSystemForm := runInitSystemForm
	oldRunHAForm := runHAForm
	defer func() {
		runHostInfoForm = oldRunHostInfoForm
		runWOLForm = oldRunWOLForm
		runInitSystemForm = oldRunInitSystemForm
		runHAForm = oldRunHAForm
	}()

	ctx, cancel := context.WithCancel(context.Background())

	runInitSystemForm = func(io []string) (initSystemResults, error) { return initSystemResults{}, nil }
	runHostInfoForm = func(resolver SystemResolver, io []huh.Option[string], im map[string]net.Interface, h string) (hostInfoResults, []huh.Option[string], error) {
		return hostInfoResults{MACAddress: "00:11:22:33:44:55"}, nil, nil
	}
	runWOLForm = func(bo []huh.Option[string]) (wolResults, error) { cancel(); return wolResults{WOLPort: "9"}, nil }
	runHAForm = func(u string) (haResults, error) { return haResults{}, nil }

	deps := setupSurveyDeps(t)
	deps.SystemResolver = &mockSystemResolver{
		discoverHomeAssistantFunc: func(c context.Context) (string, error) { <-c.Done(); return "", c.Err() },
		getWOLInterfacesFunc: func() ([]net.Interface, error) {
			return []net.Interface{{Name: "eth0", HardwareAddr: net.HardwareAddr{1, 2, 3, 4, 5, 6}}}, nil
		},
		getIPv4InfoFunc: func(net.Interface) ([]string, map[string]string) { return nil, nil },
		getFQDNFunc:     func(h string) string { return h },
	}
	if _, err := generateConfigInteractive(ctx, deps); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestPrintConfigSummary(t *testing.T) {
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	cfg := &config.Config{
		Host: config.HostConfig{
			Name:             "test-name",
			Address:          "192.168.1.50",
			MACAddress:       "00:11:22:33:44:55",
			BroadcastAddress: "192.168.1.255",
			BroadcastPort:    99,
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       "http://ha.local:8123",
			WebhookID: "abcdef12345",
		},
		InitSystem: config.InitSystemConfig{
			Name: "systemd",
		},
	}

	printConfigSummary(cmd, cfg, "/etc/remote-boot-agent/config.yaml")

	out := buf.String()
	if !strings.Contains(out, "/etc/remote-boot-agent/config.yaml") {
		t.Errorf("expected config path, got %s", out)
	}
	if !strings.Contains(out, "broadcast_address: 192.168.1.255") {
		t.Errorf("expected broadcast address, got %s", out)
	}
	if !strings.Contains(out, "broadcast_port: 99") {
		t.Errorf("expected broadcast port, got %s", out)
	}
	if !strings.Contains(out, "abcd...") {
		t.Errorf("expected truncated webhook id, got %s", out)
	}
}
