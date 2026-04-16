package system

import (
	"os"
	"strings"
	"testing"
)

func TestDetectMACAddress(t *testing.T) {
	mac, err := DetectMACAddress()
	if err != nil {
		if strings.Contains(err.Error(), "no suitable MAC address found") {
			t.Skip("Skipping MAC address test: no suitable network interface found in this environment")
		} else {
			t.Fatalf("unexpected error detecting MAC: %v", err)
		}
	}

	if mac == "" {
		t.Error("expected non-empty MAC address")
	}

	// simple validation that MAC conforms to the basic shape (has colons)
	if !strings.Contains(mac, ":") {
		t.Errorf("unrecognized MAC address format: %s", mac)
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
