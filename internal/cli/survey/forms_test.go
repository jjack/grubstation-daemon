package survey

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jjack/grubstation/internal/config"
	"github.com/jjack/grubstation/internal/homeassistant"
	"github.com/spf13/cobra"
	"github.com/yarlson/tap"
)

type mockSystemResolver struct {
	discoverHomeAssistantFunc func(ctx context.Context) ([]homeassistant.ServiceInstance, error)
	detectSystemHostnameFunc  func() (string, error)
	getWOLInterfacesFunc      func() ([]net.Interface, error)
	getIPInfoFunc             func(inf net.Interface) ([]string, map[string]string)
	getFQDNFunc               func(hostname string) string
	saveConfigFunc            func(cfg *config.Config, path string) error
	discoverGrubConfigFunc    func(ctx context.Context) (string, error)
}

func (m *mockSystemResolver) DiscoverHomeAssistant(ctx context.Context) ([]homeassistant.ServiceInstance, error) {
	if m.discoverHomeAssistantFunc != nil {
		return m.discoverHomeAssistantFunc(ctx)
	}
	return []homeassistant.ServiceInstance{{Name: "Home", URLs: []string{"http://hass.local:8123"}}}, nil
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

func (m *mockSystemResolver) GetIPInfo(inf net.Interface) ([]string, map[string]string) {
	if m.getIPInfoFunc != nil {
		return m.getIPInfoFunc(inf)
	}
	return []string{"192.168.1.100", "fd00::1"}, map[string]string{"192.168.1.100": "192.168.1.255"}
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
	resolver     *mockSystemResolver
	services     []string
	installed    bool
	installedErr error
}

func (m *mockSurveyDeps) GetSystemResolver() SystemResolver { return m.resolver }
func (m *mockSurveyDeps) IsInstalled(ctx context.Context) (bool, error) {
	return m.installed, m.installedErr
}
func (m *mockSurveyDeps) GetSupportedServices(ctx context.Context) []string { return m.services }

func setupSurveyDeps(t *testing.T) *mockSurveyDeps {
	return &mockSurveyDeps{
		resolver: &mockSystemResolver{},
	}
}

func TestGenerateConfigSurvey_Success(t *testing.T) {
	t.Setenv("GRUBSTATION_SKIP_PORT_CHECK", "true")
	t.Setenv("GRUBSTATION_SKIP_HA_URL_CHECK", "true")
	ctx := context.Background()
	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	go func() {
		// Small delay to allow the first prompt to start
		time.Sleep(50 * time.Millisecond)

		// 1. Installation Mode: Select first option (DaemonBoth)
		in.EmitKeypress("", tap.Key{Name: "return"})
		time.Sleep(20 * time.Millisecond)

		// 2. Network Interface: Select first option
		in.EmitKeypress("", tap.Key{Name: "return"})
		time.Sleep(20 * time.Millisecond)

		// 3. Host Address: Select first option (detected-host.local)
		in.EmitKeypress("", tap.Key{Name: "return"})
		time.Sleep(20 * time.Millisecond)

		// 4. Daemon Port: Default "8081"
		in.EmitKeypress("", tap.Key{Name: "return"})
		time.Sleep(20 * time.Millisecond)

		// 5. WOL Address: Select first option
		in.EmitKeypress("", tap.Key{Name: "return"})
		time.Sleep(20 * time.Millisecond)

		// 6. GRUB Wait Time: Default "2"
		in.EmitKeypress("", tap.Key{Name: "return"})
		time.Sleep(20 * time.Millisecond)

		// 7. HA URL: Default (auto-discovered)
		in.EmitKeypress("", tap.Key{Name: "return"})
		time.Sleep(20 * time.Millisecond)

		// 8. HA Webhook: Type ID
		webhook := strings.Repeat("a", 64)
		for _, r := range webhook {
			in.EmitKeypress(string(r), tap.Key{})
		}
		in.EmitKeypress("", tap.Key{Name: "return"})
	}()

	deps := setupSurveyDeps(t)
	cfg, isDryRun, err := generateConfigInteractive(ctx, deps, false, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if isDryRun {
		t.Errorf("expected isDryRun to be false")
	}

	if cfg.Host.Address != "detected-host.local" {
		t.Errorf("expected address detected-host.local, got %s", cfg.Host.Address)
	}
}

func TestGenerateConfigSurvey_MultipleHA(t *testing.T) {
	t.Setenv("GRUBSTATION_SKIP_PORT_CHECK", "true")
	t.Setenv("GRUBSTATION_SKIP_HA_URL_CHECK", "true")
	ctx := context.Background()
	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	go func() {
		time.Sleep(50 * time.Millisecond)

		for i := 0; i < 6; i++ {
			in.EmitKeypress("", tap.Key{Name: "return"})
			time.Sleep(20 * time.Millisecond)
		}

		// 7. HA Instance Selection: Select second option (Home2)
		in.EmitKeypress("", tap.Key{Name: "down"})
		time.Sleep(20 * time.Millisecond)
		in.EmitKeypress("", tap.Key{Name: "return"})
		time.Sleep(20 * time.Millisecond)

		// 7.1 HA Agent URL Selection: Select first option (http://ha2.local:8123)
		in.EmitKeypress("", tap.Key{Name: "return"})
		time.Sleep(20 * time.Millisecond)

		// 8. HA Webhook: Type ID
		webhook := strings.Repeat("a", 64)
		for _, r := range webhook {
			in.EmitKeypress(string(r), tap.Key{})
		}
		in.EmitKeypress("", tap.Key{Name: "return"})
	}()

	deps := setupSurveyDeps(t)
	deps.resolver.discoverHomeAssistantFunc = func(ctx context.Context) ([]homeassistant.ServiceInstance, error) {
		return []homeassistant.ServiceInstance{
			{Name: "Home1", URLs: []string{"http://ha1.local:8123"}},
			{Name: "Home2", URLs: []string{"http://ha2.local:8123"}},
		}, nil
	}

	cfg, _, err := generateConfigInteractive(ctx, deps, false, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HomeAssistant.URL != "http://ha2.local:8123" {
		t.Errorf("expected http://ha2.local:8123, got %s", cfg.HomeAssistant.URL)
	}
}

func TestGenerateConfigSurvey_HTTPS_HA(t *testing.T) {
	t.Setenv("GRUBSTATION_SKIP_PORT_CHECK", "true")
	t.Setenv("GRUBSTATION_SKIP_HA_URL_CHECK", "true")
	ctx := context.Background()
	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	go func() {
		time.Sleep(50 * time.Millisecond)
		for i := 0; i < 7; i++ {
			in.EmitKeypress("", tap.Key{Name: "return"})
			time.Sleep(20 * time.Millisecond)
		}

		// 8. Instance Selection: Select first option (Home)
		in.EmitKeypress("", tap.Key{Name: "return"})
		time.Sleep(20 * time.Millisecond)

		// 8.1 Agent URL Selection: Select first option (https://hass.plzwrk.net)
		in.EmitKeypress("", tap.Key{Name: "return"})
		time.Sleep(20 * time.Millisecond)

		// 8.2 GRUB URL Selection: Select first option (http IP)
		in.EmitKeypress("", tap.Key{Name: "return"})
		time.Sleep(20 * time.Millisecond)

		// 9. HA Webhook: Type ID
		webhook := strings.Repeat("a", 64)
		for _, r := range webhook {
			in.EmitKeypress(string(r), tap.Key{})
		}
		in.EmitKeypress("", tap.Key{Name: "return"})
	}()

	deps := setupSurveyDeps(t)
	deps.resolver.discoverHomeAssistantFunc = func(ctx context.Context) ([]homeassistant.ServiceInstance, error) {
		return []homeassistant.ServiceInstance{
			{
				Name: "Home",
				URLs: []string{"https://hass.plzwrk.net", "http://10.15.0.53:8123"},
			},
		}, nil
	}

	cfg, _, err := generateConfigInteractive(ctx, deps, false, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HomeAssistant.URL != "https://hass.plzwrk.net" {
		t.Errorf("expected https://hass.plzwrk.net, got %s", cfg.HomeAssistant.URL)
	}
	if cfg.Grub.URL != "http://10.15.0.53:8123" {
		t.Errorf("expected http://10.15.0.53:8123 for grub, got %s", cfg.Grub.URL)
	}
}

func TestGenerateConfigSurvey_OverwriteAbort(t *testing.T) {
	ctx := context.Background()
	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	go func() {
		time.Sleep(50 * time.Millisecond)
		// Confirmation: Select No
		in.EmitKeypress("n", tap.Key{Name: "n"})
		in.EmitKeypress("", tap.Key{Name: "return"})
	}()

	deps := setupSurveyDeps(t)
	deps.installed = true
	_, _, err := generateConfigInteractive(ctx, deps, false, 0)
	if !errors.Is(err, ErrAborted) {
		t.Errorf("expected ErrAborted, got %v", err)
	}
}

func TestBuildIfaceOptions(t *testing.T) {
	resolver := &mockSystemResolver{
		getIPInfoFunc: func(inf net.Interface) ([]string, map[string]string) {
			return []string{"192.168.1.50"}, nil
		},
	}
	mac, _ := net.ParseMAC("00:11:22:33:44:55")
	ifaces := []net.Interface{
		{Name: "eth0", HardwareAddr: mac},
	}

	opts := buildIfaceOptions(resolver, ifaces)
	if len(opts) != 1 {
		t.Fatalf("expected 1 option, got %d", len(opts))
	}

	expectedLabel := "eth0"
	expectedHint := "(00:11:22:33:44:55) [192.168.1.50]"
	if opts[0].Label != expectedLabel {
		t.Errorf("expected label %s, got %s", expectedLabel, opts[0].Label)
	}
	if opts[0].Hint != expectedHint {
		t.Errorf("expected hint %s, got %s", expectedHint, opts[0].Hint)
	}
}

func TestBuildHostSelectOptions(t *testing.T) {
	opts := buildHostSelectOptions("my-host", "my-host.local", []string{"192.168.1.50", "fd00::1"})

	if len(opts) != 4 {
		t.Fatalf("expected 4 options, got %d", len(opts))
	}
	if opts[0].Value != "my-host.local" {
		t.Errorf("expected option 0 value to be my-host.local")
	}
	if opts[2].Value != "192.168.1.50" {
		t.Errorf("expected option 2 value to be 192.168.1.50")
	}
}

func TestBuildWolSelectOptions(t *testing.T) {
	ips := []string{"192.168.1.50", "10.0.0.50"}
	broadcasts := map[string]string{
		"192.168.1.50": "192.168.1.255",
		"10.0.0.50":    "10.0.0.255",
	}

	opts := buildWolSelectOptions("192.168.1.50", ips, broadcasts)

	if len(opts) != 3 {
		t.Fatalf("expected 3 options, got %d", len(opts))
	}
	if opts[0].Value != config.DefaultWolBroadcastAddress {
		t.Errorf("expected DefaultWolBroadcastAddress, got %s", opts[0].Value)
	}
	if opts[1].Value != "192.168.1.255" {
		t.Errorf("expected subnet broadcast 192.168.1.255, got %s", opts[1].Value)
	}
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name        string
		port        string
		isReinstall bool
		currentPort int
		wantErr     bool
	}{
		{"empty", "", false, 0, true},
		{"not a number", "abc", false, 0, true},
		{"too low", "0", false, 0, true},
		{"too high", "65536", false, 0, true},
		{"reinstall same port", "8081", true, 8081, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePort(tt.port, tt.isReinstall, tt.currentPort)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePort() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPrintConfigSummary(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	tapOut := tap.NewMockWritable()
	tap.SetTermIO(nil, tapOut)
	defer tap.SetTermIO(nil, nil)

	cfg := &config.Config{
		Host: config.HostConfig{
			Address:    "192.168.1.50",
			MACAddress: "00:11:22:33:44:55",
		},
		WakeOnLan: &config.WakeOnLanConfig{
			Address: "192.168.1.255",
			Port:    99,
		},
		HomeAssistant: config.HomeAssistantConfig{
			URL:       "http://ha.local:8123",
			WebhookID: "abcdef12345",
		},
		Daemon: config.DaemonConfig{
			Port:              8081,
			ReportBootOptions: true,
		},
		Grub: &config.GrubConfig{
			WaitTimeSeconds: 2,
		},
	}

	PrintConfigSummary(cmd, cfg, "/etc/grubstation/config.yaml")

	out := strings.Join(tapOut.Buffer, "")
	if !strings.Contains(out, "/etc/grubstation/config.yaml") {
		t.Errorf("expected config path, got %s", out)
	}
	if !strings.Contains(out, "abcd...") {
		t.Errorf("expected truncated webhook id, got %s", out)
	}
}

func TestGenerateConfigSurvey_NoGrub(t *testing.T) {
	t.Setenv("GRUBSTATION_SKIP_PORT_CHECK", "true")
	t.Setenv("GRUBSTATION_SKIP_HA_URL_CHECK", "true")
	ctx := context.Background()
	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	go func() {
		time.Sleep(50 * time.Millisecond)
		// 1. Installation Mode: Only two options, select first (DaemonShutdown)
		in.EmitKeypress("", tap.Key{Name: "return"})
		time.Sleep(20 * time.Millisecond)
		// 2-6. Basic selections
		for i := 0; i < 5; i++ {
			in.EmitKeypress("", tap.Key{Name: "return"})
			time.Sleep(20 * time.Millisecond)
		}
		// 8. HA URL
		in.EmitKeypress("", tap.Key{Name: "return"})
		time.Sleep(20 * time.Millisecond)
		// 9. HA Webhook
		webhook := strings.Repeat("a", 64)
		for _, r := range webhook {
			in.EmitKeypress(string(r), tap.Key{})
		}
		in.EmitKeypress("", tap.Key{Name: "return"})
	}()

	deps := setupSurveyDeps(t)
	deps.resolver.discoverGrubConfigFunc = func(ctx context.Context) (string, error) { return "", nil }
	cfg, _, err := generateConfigInteractive(ctx, deps, false, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Daemon.ReportBootOptions {
		t.Error("expected ReportBootOptions to be false when no GRUB config")
	}
}

func TestGenerateConfigSurvey_ManualHA(t *testing.T) {
	t.Setenv("GRUBSTATION_SKIP_PORT_CHECK", "true")
	t.Setenv("GRUBSTATION_SKIP_HA_URL_CHECK", "true")
	ctx := context.Background()
	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	go func() {
		time.Sleep(50 * time.Millisecond)
		for i := 0; i < 6; i++ {
			in.EmitKeypress("", tap.Key{Name: "return"})
			time.Sleep(20 * time.Millisecond)
		}
		// 7. HA URL: Type manually
		haURL := "http://manual.ha:8123"
		for _, r := range haURL {
			in.EmitKeypress(string(r), tap.Key{})
		}
		in.EmitKeypress("", tap.Key{Name: "return"})
		time.Sleep(20 * time.Millisecond)
		// 8. HA Webhook
		webhook := strings.Repeat("a", 64)
		for _, r := range webhook {
			in.EmitKeypress(string(r), tap.Key{})
		}
		in.EmitKeypress("", tap.Key{Name: "return"})
	}()

	deps := setupSurveyDeps(t)
	deps.resolver.discoverHomeAssistantFunc = func(ctx context.Context) ([]homeassistant.ServiceInstance, error) {
		return nil, nil // No discovered instances
	}

	cfg, _, err := generateConfigInteractive(ctx, deps, false, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HomeAssistant.URL != "http://manual.ha:8123" {
		t.Errorf("expected http://manual.ha:8123, got %s", cfg.HomeAssistant.URL)
	}
}

func TestValidatePort_InUse(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	port := l.Addr().(*net.TCPAddr).Port

	_ = os.Unsetenv("GRUBSTATION_SKIP_PORT_CHECK")
	err = validatePort(strconv.Itoa(port), false, 0)
	if err == nil {
		t.Error("expected error for in-use port, got nil")
	}
}

func TestBuildWolSelectOptions_IPv6(t *testing.T) {
	ips := []string{"192.168.1.50", "fd00::1"}
	broadcasts := map[string]string{
		"192.168.1.50": "192.168.1.255",
	}

	// Host address is IPv6, but we should still offer the IPv4 subnet broadcast
	// because WOL is an IPv4 mechanism and the interface supports it.
	opts := buildWolSelectOptions("fd00::1", ips, broadcasts)

	if len(opts) != 2 {
		t.Fatalf("expected 2 options (default + IPv4 broadcast), got %d", len(opts))
	}
}

func TestValidatePort_ReinstallDifferent(t *testing.T) {
	t.Setenv("GRUBSTATION_SKIP_PORT_CHECK", "true")
	err := validatePort("8082", true, 8081)
	if err != nil {
		t.Errorf("expected no error with skip check, got %v", err)
	}
}
