package system

import (
	"net"
	"os"
	"strings"
	"testing"
)

func TestGetInterfaceOptions(t *testing.T) {
	opts, err := GetInterfaceOptions()
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
		if !strings.Contains(opt.Value, ":") {
			t.Errorf("unrecognized MAC address format: %s", opt.Value)
		}
		if opt.Label == "" {
			t.Error("expected non-empty label")
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

func TestGetIPAddrs_ValidInterface(t *testing.T) {
	interfaces, err := net.Interfaces()
	if err != nil || len(interfaces) == 0 {
		t.Skip("No interfaces available for testing")
	}
	addrs := GetIPAddrs(interfaces[0])
	if addrs == nil {
		t.Error("expected GetIPAddrs to return a slice, got nil")
	}
}
