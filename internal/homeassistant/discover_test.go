package homeassistant

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/mdns"
)

func TestDiscover_Timeout(t *testing.T) {
	// Provide a short-circuit mock so we don't actually wait the full 5s
	oldQuery := mdnsQuery
	defer func() { mdnsQuery = oldQuery }()
	mdnsQuery = func(params *mdns.QueryParam) error {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	urls, err := Discover(ctx)
	if err != nil {
		t.Fatalf("expected no error on timeout, got %v", err)
	}
	if len(urls) > 0 {
		t.Logf("Found HA at %v", urls)
	}
}

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
			Name:       "Home." + homeAssistantService + ".local.",
			AddrV4:     net.ParseIP("192.168.1.100"),
			Port:       8123,
			InfoFields: []string{"internal_url=http://ha.local:8123"},
		}
		// Hashicorp's Query runs synchronously for the timeout duration,
		// but since we are mocking it, we should just return immediately.
		return nil
	}

	instances, err := Discover(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(instances) != 1 || instances[0].Name != "Home" {
		t.Fatalf("expected 1 instance named 'Home', got %v", instances)
	}
	urls := instances[0].URLs
	if len(urls) != 2 || urls[0] != "http://ha.local:8123" || urls[1] != "http://192.168.1.100:8123" {
		t.Errorf("expected ['http://ha.local:8123', 'http://192.168.1.100:8123'], got %v", urls)
	}
}

func TestDiscover_MultipleSuccess(t *testing.T) {
	oldNetInterfaces := netInterfaces
	defer func() { netInterfaces = oldNetInterfaces }()
	netInterfaces = func() ([]net.Interface, error) {
		return nil, nil
	}

	oldQuery := mdnsQuery
	defer func() { mdnsQuery = oldQuery }()
	var called bool
	mdnsQuery = func(params *mdns.QueryParam) error {
		// Only send on the first query to avoid duplicates if there are multiple interfaces
		if !called {
			called = true
			params.Entries <- &mdns.ServiceEntry{
				Name:       "Home1." + homeAssistantService + ".local.",
				AddrV4:     net.ParseIP("192.168.1.100"),
				Port:       8123,
				InfoFields: []string{"internal_url=http://ha1.local:8123"},
			}
			params.Entries <- &mdns.ServiceEntry{
				Name:       "Home2." + homeAssistantService + ".local.",
				AddrV4:     net.ParseIP("192.168.1.101"),
				Port:       8123,
				InfoFields: []string{"internal_url=http://ha2.local:8123"},
			}
		}
		return nil
	}

	instances, err := Discover(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(instances) != 2 {
		t.Errorf("expected 2 instances, got %d", len(instances))
	}
}

func TestDiscover_EmptyEntry(t *testing.T) {
	oldNetInterfaces := netInterfaces
	defer func() { netInterfaces = oldNetInterfaces }()
	netInterfaces = func() ([]net.Interface, error) {
		return nil, nil
	}

	oldQuery := mdnsQuery
	defer func() { mdnsQuery = oldQuery }()
	mdnsQuery = func(params *mdns.QueryParam) error {
		params.Entries <- &mdns.ServiceEntry{} // empty, should be ignored
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	urls, err := Discover(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(urls) != 0 {
		t.Errorf("expected empty urls, got %v", urls)
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
			name: "base_url present",
			entry: &mdns.ServiceEntry{
				InfoFields: []string{"base_url=http://base.local:8123"},
				AddrV4:     net.ParseIP("192.168.1.100"),
				Port:       8123,
			},
			expected: []string{"http://base.local:8123", "http://192.168.1.100:8123"},
		},
		{
			name: "empty txt records only ip",
			entry: &mdns.ServiceEntry{
				InfoFields: []string{"internal_url=", "base_url="},
				AddrV4:     net.ParseIP("192.168.1.100"),
				Port:       8123,
			},
			expected: []string{"http://192.168.1.100:8123"},
		},
		{
			name: "include https internal_url",
			entry: &mdns.ServiceEntry{
				InfoFields: []string{"internal_url=https://ha.local:8123", "base_url=http://base.local:8123"},
				AddrV4:     net.ParseIP("192.168.1.100"),
				Port:       8123,
			},
			expected: []string{"https://ha.local:8123", "http://base.local:8123", "http://192.168.1.100:8123"},
		},
		{
			name: "include https base_url",
			entry: &mdns.ServiceEntry{
				InfoFields: []string{"base_url=https://base.local:8123"},
				AddrV4:     net.ParseIP("192.168.1.100"),
				Port:       8123,
			},
			expected: []string{"https://base.local:8123", "http://192.168.1.100:8123"},
		},
		{
			name: "no txt records only ip",
			entry: &mdns.ServiceEntry{
				AddrV4: net.ParseIP("10.0.0.5"),
				Port:   8123,
			},
			expected: []string{"http://10.0.0.5:8123"},
		},
		{
			name:     "no useful info",
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
