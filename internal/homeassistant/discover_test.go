//go:build !windows

package homeassistant

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/mdns"
)

func TestDiscover_Success(t *testing.T) {
	oldNetInterfaces := netInterfaces
	defer func() { netInterfaces = oldNetInterfaces }()
	netInterfaces = func() ([]net.Interface, error) {
		return nil, nil // Return no interfaces to force a single global query
	}

	oldQuery := mdnsQuery
	defer func() { mdnsQuery = oldQuery }()
	mdnsQuery = func(params *mdns.QueryParam) error {
		params.Entries <- &mdns.ServiceEntry{
			Name:       "Home." + homeAssistantService,
			AddrV4:     net.ParseIP("192.168.1.100"),
			Port:       8123,
			InfoFields: []string{"internal_url=http://ha.local:8123"},
		}
		return nil
	}

	// Set a short timeout
	oldTimeout := discoveryTimeout
	discoveryTimeout = 10 * time.Millisecond
	defer func() { discoveryTimeout = oldTimeout }()

	instances, err := Discover(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(instances) != 1 || instances[0].Name != "Home" {
		t.Fatalf("expected 1 instance named 'Home', got %v", instances)
	}
}

func TestExtractURLs(t *testing.T) {
	tests := []struct {
		name     string
		entry    *mdns.ServiceEntry
		expected []string
	}{
		{
			name: "internal_url and ip present",
			entry: &mdns.ServiceEntry{
				InfoFields: []string{"internal_url=http://ha.local:8123", "base_url=http://base.local"},
				AddrV4:     net.ParseIP("192.168.1.100"),
				Port:       8123,
			},
			expected: []string{"http://ha.local:8123", "http://base.local", "http://192.168.1.100:8123"},
		},
		{
			name: "no useful info",
			entry:    &mdns.ServiceEntry{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urls := extractURLs(tt.entry)
			if len(urls) != len(tt.expected) {
				t.Fatalf("expected %d urls, got %d: %v", len(tt.expected), len(urls), urls)
			}
			for i := range urls {
				if urls[i] != tt.expected[i] {
					t.Errorf("at index %d: expected %s, got %s", i, tt.expected[i], urls[i])
				}
			}
		})
	}
}
