package host

import (
	"errors"
	"net"
	"os"
	"strings"
	"testing"
)

func TestGetWOLInterfaces(t *testing.T) {
	oldNetInterfaces := netInterfaces
	oldOsStat := osStat
	defer func() {
		netInterfaces = oldNetInterfaces
		osStat = oldOsStat
	}()

	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	netInterfaces = func() ([]net.Interface, error) {
		return []net.Interface{{Name: "eth0", HardwareAddr: mac, Flags: net.FlagUp}}, nil
	}

	osStat = func(name string) (os.FileInfo, error) {
		return nil, nil // mock device file exists
	}

	opts, err := GetWOLInterfaces()
	if err != nil {
		t.Fatalf("unexpected error getting interfaces: %v", err)
	}

	if len(opts) == 0 {
		t.Error("expected non-empty interface options list")
	}

	for _, opt := range opts {
		t.Logf("Found WOL capable interface: %s (%s)", opt.Name, opt.HardwareAddr.String())
		if opt.HardwareAddr.String() == "" {
			t.Errorf("expected non-empty hardware address for %s", opt.Name)
		}
		if opt.Name == "" {
			t.Error("expected non-empty name")
		}
	}
}

func TestDetectHostname(t *testing.T) {
	hostname, err := DetectHostname()
	if err != nil {
		t.Fatalf("unexpected error detecting hostname: %v", err)
	}

	if hostname == "" {
		t.Error("expected non-empty hostname")
	}

	expected, _ := os.Hostname()
	if hostname != expected {
		t.Errorf("expected hostname %s, got %s", expected, hostname)
	}
}

func TestDetectHostname_Error(t *testing.T) {
	oldOsHostname := osHostname
	defer func() { osHostname = oldOsHostname }()

	osHostname = func() (string, error) {
		return "", errors.New("mock hostname error")
	}

	_, err := DetectHostname()
	if err == nil {
		t.Fatal("expected error detecting hostname, got nil")
	}
	if !strings.Contains(err.Error(), "mock hostname error") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetWOLInterfaces_Error(t *testing.T) {
	oldNetInterfaces := netInterfaces
	defer func() { netInterfaces = oldNetInterfaces }()

	netInterfaces = func() ([]net.Interface, error) {
		return nil, errors.New("mock interfaces error")
	}

	_, err := GetWOLInterfaces()
	if err == nil {
		t.Fatal("expected error getting interface options, got nil")
	}
	if !strings.Contains(err.Error(), "mock interfaces error") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetWOLInterfaces_NoSuitable(t *testing.T) {
	oldNetInterfaces := netInterfaces
	defer func() { netInterfaces = oldNetInterfaces }()

	netInterfaces = func() ([]net.Interface, error) {
		// Return an interface that will be skipped (no MAC)
		return []net.Interface{
			{
				Name:         "lo",
				HardwareAddr: nil,
				Flags:        net.FlagUp | net.FlagLoopback,
			},
		}, nil
	}

	_, err := GetWOLInterfaces()
	if err == nil {
		t.Fatal("expected error for no suitable interfaces, got nil")
	}
	if !strings.Contains(err.Error(), "no suitable interfaces found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestIsWOLCapableInterface(t *testing.T) {
	mac, _ := net.ParseMAC("00:11:22:33:44:55")

	oldOsStat := osStat
	defer func() { osStat = oldOsStat }()

	osStat = func(name string) (os.FileInfo, error) {
		return nil, nil // mock device file exists
	}

	tests := []struct {
		name     string
		inf      net.Interface
		expected bool
	}{
		{
			name:     "no mac",
			inf:      net.Interface{Flags: net.FlagUp},
			expected: false,
		},
		{
			name:     "not up",
			inf:      net.Interface{HardwareAddr: mac, Flags: 0},
			expected: false,
		},
		{
			name:     "loopback",
			inf:      net.Interface{HardwareAddr: mac, Flags: net.FlagUp | net.FlagLoopback},
			expected: false,
		},
		{
			name:     "virtual prefix",
			inf:      net.Interface{Name: "docker0", HardwareAddr: mac, Flags: net.FlagUp},
			expected: false,
		},
		{
			name:     "valid interface",
			inf:      net.Interface{Name: "eth0", HardwareAddr: mac, Flags: net.FlagUp},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isWOLCapableInterface(tt.inf); got != tt.expected {
				t.Errorf("isWOLCapableInterface() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetIPv4Info(t *testing.T) {
	oldGetAddrs := getAddrs
	defer func() { getAddrs = oldGetAddrs }()

	getAddrs = func(iface net.Interface) ([]net.Addr, error) {
		ip, ipnet, _ := net.ParseCIDR("192.168.1.50/24")
		ipnet.IP = ip
		return []net.Addr{ipnet}, nil
	}

	ips, broadcasts := GetIPInfo(net.Interface{Name: "eth0"})
	if len(ips) != 1 || ips[0] != "192.168.1.50" {
		t.Errorf("expected ips to contain 192.168.1.50, got %v", ips)
	}
	if broadcasts["192.168.1.50"] != "192.168.1.255" {
		t.Errorf("expected broadcast to be 192.168.1.255, got %s", broadcasts["192.168.1.50"])
	}
}

func TestGetIPv4Info_Error(t *testing.T) {
	oldGetAddrs := getAddrs
	defer func() { getAddrs = oldGetAddrs }()

	getAddrs = func(iface net.Interface) ([]net.Addr, error) {
		return nil, errors.New("mock addrs error")
	}

	ips, broadcasts := GetIPInfo(net.Interface{Name: "eth0"})
	if len(ips) != 0 {
		t.Errorf("expected no ips on error, got %v", ips)
	}
	if len(broadcasts) != 0 {
		t.Errorf("expected no broadcasts on error, got %v", broadcasts)
	}
}

func TestGetIPv4Info_IPv6(t *testing.T) {
	oldGetAddrs := getAddrs
	defer func() { getAddrs = oldGetAddrs }()

	getAddrs = func(iface net.Interface) ([]net.Addr, error) {
		ip, ipnet, _ := net.ParseCIDR("2001:db8::1/64")
		ipnet.IP = ip
		return []net.Addr{ipnet}, nil
	}

	ips, broadcasts := GetIPInfo(net.Interface{Name: "eth0"})
	if len(ips) != 1 || ips[0] != "2001:db8::1" {
		t.Errorf("expected ips to contain 2001:db8::1, got %v", ips)
	}
	if broadcasts["2001:db8::1"] != "" {
		t.Errorf("expected NO broadcast to be calculated for IPv6")
	}
}

func TestGetIPv4Info_NonIPNet(t *testing.T) {
	oldGetAddrs := getAddrs
	defer func() { getAddrs = oldGetAddrs }()

	getAddrs = func(iface net.Interface) ([]net.Addr, error) {
		return []net.Addr{&net.UnixAddr{Name: "test", Net: "unix"}}, nil
	}

	ips, broadcasts := GetIPInfo(net.Interface{Name: "eth0"})
	if len(ips) != 0 {
		t.Errorf("expected 0 ips, got %d", len(ips))
	}
	if len(broadcasts) != 0 {
		t.Errorf("expected 0 broadcasts, got %d", len(broadcasts))
	}
}

func TestGetFQDN(t *testing.T) {
	oldLookupCNAME := netLookupCNAME
	defer func() { netLookupCNAME = oldLookupCNAME }()

	netLookupCNAME = func(name string) (string, error) {
		return name + ".local.lan.", nil
	}

	fqdn := GetFQDN("my-host")
	if fqdn != "my-host.local.lan" {
		t.Errorf("expected my-host.local.lan, got %s", fqdn)
	}

	netLookupCNAME = func(name string) (string, error) {
		return "", errors.New("lookup failed")
	}

	fqdn = GetFQDN("my-host")
	if fqdn != "my-host" {
		t.Errorf("expected fallback to short hostname 'my-host', got %s", fqdn)
	}
}
