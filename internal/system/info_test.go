package system

import (
	"errors"
	"net"
	"os"
	"strings"
	"testing"
)

func TestGetWOLInterfaces(t *testing.T) {
	opts, err := GetWOLInterfaces()
	if err != nil {
		if strings.Contains(err.Error(), "no suitable interfaces found") {
			t.Skip("Skipping interface test: no suitable network interface found in this environment")
		} else {
			t.Fatalf("unexpected error getting interfaces: %v", err)
		}
	}

	if len(opts) == 0 {
		t.Error("expected non-empty interface options list")
	}

	for _, opt := range opts {
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

func TestGetIPv4Info(t *testing.T) {
	oldGetAddrs := getAddrs
	defer func() { getAddrs = oldGetAddrs }()

	getAddrs = func(iface net.Interface) ([]net.Addr, error) {
		ip, ipnet, _ := net.ParseCIDR("192.168.1.50/24")
		ipnet.IP = ip
		return []net.Addr{ipnet}, nil
	}

	ips, broadcasts := GetIPv4Info(net.Interface{Name: "eth0"})
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

	ips, broadcasts := GetIPv4Info(net.Interface{Name: "eth0"})
	if len(ips) != 0 {
		t.Errorf("expected no ips on error, got %v", ips)
	}
	if len(broadcasts) != 0 {
		t.Errorf("expected no broadcasts on error, got %v", broadcasts)
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
