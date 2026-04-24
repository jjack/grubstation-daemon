package system

import (
	"os"
	"strings"
	"testing"
)

func TestDetectMACAddress(t *testing.T) {
	macs, err := DetectMACAddresses()
	if err != nil {
		if strings.Contains(err.Error(), "no suitable MAC address found") {
			t.Skip("Skipping MAC address test: no suitable network interface found in this environment")
		} else {
			t.Fatalf("unexpected error detecting MAC: %v", err)
		}
	}

	if len(macs) == 0 {
		t.Error("expected non-empty MAC address list")
	}

	// simple validation that MACs conform to the basic shape (has colons)
	for _, mac := range macs {
		if !strings.Contains(mac.HardwareAddr.String(), ":") {
			t.Errorf("unrecognized MAC address format: %s", mac.HardwareAddr.String())
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
	macs, err := DetectMACAddresses()
	if err != nil || len(macs) == 0 {
		t.Skip("No interfaces available for testing")
	}
	addrs := GetIPAddrs(macs[0])
	if addrs == nil {
		t.Error("expected GetIPAddrs to return a slice, got nil")
	}
}

func TestGetInterfaceOptions_AlwaysReturnsSlice(t *testing.T) {
	opts, err := GetInterfaceOptions()
	if err != nil {
		t.Fatalf("unexpected error getting interface options: %v", err)
	}
	if opts == nil {
		t.Error("expected GetInterfaceOptions to return a slice, got nil")
	}
}
