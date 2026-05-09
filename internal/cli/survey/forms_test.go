package survey

import (
	"bytes"
	"context"
	"errors"
	"net"
	"runtime"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/jjack/grubstation-cli/internal/config"
	"github.com/spf13/cobra"
)

type mockSystemResolver struct {
	discoverHomeAssistantFunc func(ctx context.Context) (string, error)
	detectSystemHostnameFunc  func() (string, error)
	getWOLInterfacesFunc      func() ([]net.Interface, error)
	getIPv4InfoFunc           func(inf net.Interface) ([]string, map[string]string)
	getFQDNFunc               func(hostname string) string
	saveConfigFunc            func(cfg *config.Config, path string) error
	discoverGrubConfigFunc    func(ctx context.Context) (string, error)
}

func (m *mockSystemResolver) DiscoverHomeAssistant(ctx context.Context) (string, error) {
	if m.discoverHomeAssistantFunc != nil {
		return m.discoverHomeAssistantFunc(ctx)
	}
	return "http://hass.local:8123", nil
}

func (m *mockSystemResolver) DiscoverGrubConfig(ctx context.Context) (string, error) {
	if m.discoverGrubConfigFunc != nil {
		return m.discoverGrubConfigFunc(ctx)
	}
	return "/boot/grub/grub.cfg", nil
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

type mockSurveyDeps struct {
	resolver *mockSystemResolver
	services []string
}

func (m *mockSurveyDeps) GetSystemResolver() SystemResolver                 { return m.resolver }
func (m *mockSurveyDeps) GetSupportedServices(ctx context.Context) []string { return m.services }

func setupSurveyDeps(t *testing.T) *mockSurveyDeps {
	return &mockSurveyDeps{
		resolver: &mockSystemResolver{},
	}
}

func mockAllForms() func() {
	oldNetworking := runNetworkingIfaceForm
	oldHostInfo := runHostInfoForm
	oldAgentConfig := runAgentConfigForm
	oldWOL := runWOLForm
	oldHA := runHAForm

	runNetworkingIfaceForm = func(ifaceOpts []huh.Option[string]) (string, error) { return "eth0", nil }
	runHostInfoForm = func(hostOpts []huh.Option[string], hostname string) (hostInfoResults, error) {
		return hostInfoResults{Name: "test-name", HostAddress: "192.168.1.100"}, nil
	}
	runAgentConfigForm = func(hasGrub bool) (agentConfigResults, error) {
		return agentConfigResults{Mode: ModeDaemonBoth, DaemonPort: "8081", GrubWaitTimeSeconds: "2"}, nil
	}
	runWOLForm = func(bo []huh.Option[string]) (wolResults, error) {
		return wolResults{Broadcast: "192.168.1.255", WOLPort: "9"}, nil
	}
	runHAForm = func(u string) (haResults, error) {
		return haResults{URL: "http://hass.local:8123", WebhookID: "webhook123"}, nil
	}

	return func() {
		runNetworkingIfaceForm = oldNetworking
		runHostInfoForm = oldHostInfo
		runAgentConfigForm = oldAgentConfig
		runWOLForm = oldWOL
		runHAForm = oldHA
	}
}

func TestGenerateConfigSurvey_Success(t *testing.T) {
	defer mockAllForms()()

	deps := setupSurveyDeps(t)
	deps.resolver = &mockSystemResolver{
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
	if cfg.WakeOnLan.Address != "192.168.1.255" {
		t.Errorf("expected Address 192.168.1.255, got %s", cfg.WakeOnLan.Address)
	}
	if cfg.WakeOnLan.Port != 9 {
		t.Errorf("expected Port 9 (fallback), got %d", cfg.WakeOnLan.Port)
	}
	if cfg.Daemon.ListenPort != 8081 {
		t.Errorf("expected ListenPort 8081, got %d", cfg.Daemon.ListenPort)
	}
	if cfg.HomeAssistant.URL != "http://hass.local:8123" {
		t.Errorf("expected URL http://hass.local:8123, got %s", cfg.HomeAssistant.URL)
	}
}

func TestGenerateConfigSurvey_FormErrors(t *testing.T) {
	defer mockAllForms()()
	deps := setupSurveyDeps(t)

	resetMocks := func() {
		runNetworkingIfaceForm = func(ifaceOpts []huh.Option[string]) (string, error) { return "eth0", nil }
		runHostInfoForm = func(hostOpts []huh.Option[string], hostname string) (hostInfoResults, error) {
			return hostInfoResults{
				Name:        "test-name",
				HostAddress: "192.168.1.100",
			}, nil
		}
		runAgentConfigForm = func(hasGrub bool) (agentConfigResults, error) {
			return agentConfigResults{
				Mode:                ModeDaemonBoth,
				DaemonPort:          "8081",
				GrubWaitTimeSeconds: "2",
			}, nil
		}
		runWOLForm = func(bo []huh.Option[string]) (wolResults, error) {
			return wolResults{Broadcast: "192.168.1.255", WOLPort: "9"}, nil
		}
		runHAForm = func(u string) (haResults, error) {
			return haResults{URL: "http://hass.local:8123", WebhookID: "webhook123"}, nil
		}
	}

	t.Run("Networking Iface Form Error", func(t *testing.T) {
		runNetworkingIfaceForm = func(ifaceOpts []huh.Option[string]) (string, error) {
			return "", errors.New("simulated iface error")
		}
		_, err := generateConfigInteractive(context.Background(), deps)
		if err == nil || err.Error() != "simulated iface error" {
			t.Fatalf("expected simulated iface error, got %v", err)
		}
		resetMocks()
	})

	t.Run("Host Info Form Error", func(t *testing.T) {
		runHostInfoForm = func(hostOpts []huh.Option[string], hostname string) (hostInfoResults, error) {
			return hostInfoResults{}, errors.New("simulated host info error")
		}
		_, err := generateConfigInteractive(context.Background(), deps)
		if err == nil || err.Error() != "simulated host info error" {
			t.Fatalf("expected simulated host info error, got %v", err)
		}
		resetMocks()
	})

	t.Run("Agent Config Form Error", func(t *testing.T) {
		runAgentConfigForm = func(hasGrub bool) (agentConfigResults, error) {
			return agentConfigResults{}, errors.New("simulated agent config error")
		}
		_, err := generateConfigInteractive(context.Background(), deps)
		if err == nil || err.Error() != "simulated agent config error" {
			t.Fatalf("expected simulated agent config error, got %v", err)
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
		d.resolver = &mockSystemResolver{
			detectSystemHostnameFunc: func() (string, error) { return "", errors.New("simulated hostname error") },
		}
		_, err := generateConfigInteractive(context.Background(), d)
		if err == nil || err.Error() != "simulated hostname error" {
			t.Fatalf("expected simulated hostname error, got %v", err)
		}
	})

	t.Run("Get WOL Interfaces Error", func(t *testing.T) {
		d := setupSurveyDeps(t)
		d.resolver = &mockSystemResolver{
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
		defer mockAllForms()()

		deps := setupSurveyDeps(t)
		deps.resolver = &mockSystemResolver{
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
	if opts[0].Value != "my-host.local" {
		t.Errorf("expected option 0 to be my-host.local")
	}
	if opts[1].Value != "my-host" {
		t.Errorf("expected option 1 to be my-host")
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

func TestBuildWolOptions(t *testing.T) {
	ips := []string{"192.168.1.50", "10.0.0.50"}
	broadcasts := map[string]string{
		"192.168.1.50": "192.168.1.255",
		"10.0.0.50":    "10.0.0.255",
	}

	opts := buildWolOptions("192.168.1.50", ips, broadcasts)

	if len(opts) != 3 {
		t.Fatalf("expected 3 options, got %d", len(opts))
	}
	if opts[0].Value != config.DefaultWolAddress {
		t.Errorf("expected DefaultWolAddress, got %s", opts[0].Value)
	}
	if opts[1].Value != "192.168.1.255" {
		t.Errorf("expected subnet broadcast 192.168.1.255, got %s", opts[1].Value)
	}
	if opts[2].Value != "10.0.0.255" {
		t.Errorf("expected subnet broadcast 10.0.0.255, got %s", opts[2].Value)
	}

	// Test deduplication
	ipsDup := []string{"192.168.1.50", "192.168.1.51"}
	broadcastsDup := map[string]string{
		"192.168.1.50": "192.168.1.255",
		"192.168.1.51": "192.168.1.255",
	}
	optsDup := buildWolOptions("192.168.1.50", ipsDup, broadcastsDup)
	if len(optsDup) != 2 {
		t.Fatalf("expected 2 options due to dedup, got %d", len(optsDup))
	}

	// Test IPv6 filtering (if HostAddress is IPv4, it filters out IPv6 subnets)
	ipsMix := []string{"192.168.1.50", "fe80::1"}
	broadcastsMix := map[string]string{
		"192.168.1.50": "192.168.1.255",
		"fe80::1":      "fe80::ffff",
	}
	optsMix := buildWolOptions("192.168.1.50", ipsMix, broadcastsMix)
	if len(optsMix) != 2 {
		t.Fatalf("expected 2 options due to ipv6 filtering, got %d", len(optsMix))
	}

	// Test non-IP host address (should not filter)
	optsNonIP := buildWolOptions("my-host.local", ipsMix, broadcastsMix)
	if len(optsNonIP) != 3 {
		t.Fatalf("expected 3 options when host address is not an IP, got %d", len(optsNonIP))
	}
}

func TestGenerateConfigSurvey_ContextCancelBeforeHA(t *testing.T) {
	defer mockAllForms()()

	ctx, cancel := context.WithCancel(context.Background())

	runHostInfoForm = func(hostOpts []huh.Option[string], hostname string) (hostInfoResults, error) {
		cancel()
		return hostInfoResults{}, nil
	}

	deps := setupSurveyDeps(t)
	deps.resolver = &mockSystemResolver{
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
			Name:       "test-name",
			Address:    "192.168.1.50",
			MACAddress: "00:11:22:33:44:55",
		},
		WakeOnLan: config.WakeOnLanConfig{
			Address: "192.168.1.255",
			Port:    99,
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       "http://ha.local:8123",
			WebhookID: "abcdef12345",
		},
		Daemon: config.DaemonConfig{
			ListenPort:        8081,
			ReportBootOptions: true,
		},
		Grub: config.GrubConfig{
			WaitTimeSeconds: 2,
		},
	}

	PrintConfigSummary(cmd, cfg, "/etc/grubstation/config.yaml")

	out := buf.String()
	if !strings.Contains(out, "/etc/grubstation/config.yaml") {
		t.Errorf("expected config path, got %s", out)
	}
	if !strings.Contains(out, "address: 192.168.1.255") {
		t.Errorf("expected broadcast address, got %s", out)
	}
	if !strings.Contains(out, "port: 99") {
		t.Errorf("expected broadcast port, got %s", out)
	}
	if !strings.Contains(out, "abcd...") {
		t.Errorf("expected truncated webhook id, got %s", out)
	}
	if runtime.GOOS == "linux" && !strings.Contains(out, "grub:\n  wait_time_seconds: 2") {
		t.Errorf("expected grub wait time in summary, got %s", out)
	}
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		port    string
		wantErr bool
	}{
		{"empty", "", true},
		{"not a number", "abc", true},
		{"too low", "0", true},
		{"too high", "65536", true},
		{"valid", "8081", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePort(tt.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePort() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	// Test port in use
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on random port: %v", err)
	}
	defer l.Close()
	_, portStr, _ := net.SplitHostPort(l.Addr().String())

	err = validatePort(portStr)
	if err == nil {
		t.Errorf("expected error for port in use, got nil")
	}
}

func TestGenerateConfigSurvey_IPv6(t *testing.T) {
	defer mockAllForms()()

	wolFormCalled := false
	runWOLForm = func(bo []huh.Option[string]) (wolResults, error) {
		wolFormCalled = true
		return wolResults{Broadcast: "255.255.255.255", WOLPort: "9"}, nil
	}

	runHostInfoForm = func(hostOpts []huh.Option[string], hostname string) (hostInfoResults, error) {
		return hostInfoResults{Name: "test", HostAddress: "fd00::1"}, nil
	}

	deps := setupSurveyDeps(t)
	cfg, err := generateConfigInteractive(context.Background(), deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wolFormCalled {
		t.Error("expected WOL form to be skipped for IPv6")
	}
	if cfg.WakeOnLan.Address != "fd00::1" {
		t.Errorf("expected broadcast to be host address for IPv6, got %s", cfg.WakeOnLan.Address)
	}
}

func TestGenerateConfigSurvey_NoBootOptions(t *testing.T) {
	defer mockAllForms()()

	runAgentConfigForm = func(hasGrub bool) (agentConfigResults, error) {
		return agentConfigResults{Mode: ModeDaemonShutdown, DaemonPort: "8081"}, nil
	}
	wolFormCalled := false
	runWOLForm = func(bo []huh.Option[string]) (wolResults, error) {
		wolFormCalled = true
		return wolResults{Broadcast: "255.255.255.255", WOLPort: "9"}, nil
	}

	deps := setupSurveyDeps(t)
	cfg, err := generateConfigInteractive(context.Background(), deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if runtime.GOOS == "linux" {
		if wolFormCalled {
			t.Error("expected WOL form to be skipped on linux when boot options disabled")
		}
		if cfg.WakeOnLan.Address != config.DefaultWolAddress {
			t.Errorf("expected default WOL address, got %s", cfg.WakeOnLan.Address)
		}
	} else if runtime.GOOS == "windows" {
		if !wolFormCalled {
			t.Error("expected WOL form to be called on windows")
		}
	}

	if cfg.Daemon.ReportBootOptions {
		t.Error("expected ReportBootOptions to be false")
	}
}

func TestGenerateConfigSurvey_SingleWolOption(t *testing.T) {
	defer mockAllForms()()

	wolFormCalled := false
	runWOLForm = func(bo []huh.Option[string]) (wolResults, error) {
		wolFormCalled = true
		return wolResults{Broadcast: "255.255.255.255", WOLPort: "9"}, nil
	}

	deps := setupSurveyDeps(t)
	deps.resolver = &mockSystemResolver{
		getIPv4InfoFunc: func(inf net.Interface) ([]string, map[string]string) {
			return []string{"192.168.1.50"}, nil
		},
	}

	cfg, err := generateConfigInteractive(context.Background(), deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wolFormCalled {
		t.Error("expected WOL form to be skipped when only 1 option available")
	}
	if cfg.WakeOnLan.Address != config.DefaultWolAddress {
		t.Errorf("expected default WOL address, got %s", cfg.WakeOnLan.Address)
	}
}

func TestGenerateConfigSurvey_NoGrub(t *testing.T) {
	defer mockAllForms()()

	var passedHasGrub bool
	runAgentConfigForm = func(hasGrub bool) (agentConfigResults, error) {
		passedHasGrub = hasGrub
		return agentConfigResults{Mode: ModeDaemonShutdown, DaemonPort: "8081"}, nil
	}

	deps := setupSurveyDeps(t)
	deps.resolver = &mockSystemResolver{
		discoverGrubConfigFunc: func(ctx context.Context) (string, error) {
			return "", errors.New("no grub")
		},
	}

	cfg, err := generateConfigInteractive(context.Background(), deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if runtime.GOOS == "linux" && passedHasGrub {
		t.Error("expected hasGrub to be false on linux when discovery fails")
	}
	if cfg.Grub.ConfigPath != "" {
		t.Errorf("expected empty grub config path, got %s", cfg.Grub.ConfigPath)
	}
}

func TestGenerateConfigSurvey_HookOnly(t *testing.T) {
	defer mockAllForms()()

	runAgentConfigForm = func(hasGrub bool) (agentConfigResults, error) {
		return agentConfigResults{Mode: ModeHookOnly, DaemonPort: "8081", GrubWaitTimeSeconds: "5"}, nil
	}

	deps := setupSurveyDeps(t)
	cfg, err := generateConfigInteractive(context.Background(), deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Daemon.ListenPort != 0 {
		t.Errorf("expected port 0 for hook-only mode, got %d", cfg.Daemon.ListenPort)
	}
	if !cfg.Daemon.ReportBootOptions {
		t.Error("expected report boot options to be true")
	}
	if cfg.Grub.WaitTimeSeconds != 5 {
		t.Errorf("expected wait time 5, got %d", cfg.Grub.WaitTimeSeconds)
	}
}
