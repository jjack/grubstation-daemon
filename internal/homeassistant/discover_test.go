package homeassistant

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/grandcat/zeroconf"
)

func TestDiscover_Timeout(t *testing.T) {
	// Without a zeroconf server, this will timeout and return an empty string.
	url, err := Discover(context.Background())
	if err != nil {
		t.Fatalf("expected no error on timeout, got %v", err)
	}
	if url != "" {
		t.Logf("Found HA at %s", url)
	}
}

type mockResolver struct {
	browseFunc func(ctx context.Context, service string, domain string, entries chan<- *zeroconf.ServiceEntry) error
}

func (m *mockResolver) Browse(ctx context.Context, service string, domain string, entries chan<- *zeroconf.ServiceEntry) error {
	return m.browseFunc(ctx, service, domain, entries)
}

func TestDiscover_NewResolverError(t *testing.T) {
	oldNewResolver := newResolver
	defer func() { newResolver = oldNewResolver }()
	newResolver = func() (mdnsResolver, error) {
		return nil, errors.New("mock resolver error")
	}

	_, err := Discover(context.Background())
	if err == nil || err.Error() != "mock resolver error" {
		t.Fatalf("expected 'mock resolver error', got %v", err)
	}
}

func TestDiscover_BrowseError(t *testing.T) {
	oldNewResolver := newResolver
	defer func() { newResolver = oldNewResolver }()
	newResolver = func() (mdnsResolver, error) {
		return &mockResolver{
			browseFunc: func(ctx context.Context, service string, domain string, entries chan<- *zeroconf.ServiceEntry) error {
				return errors.New("mock browse error")
			},
		}, nil
	}

	_, err := Discover(context.Background())
	if err == nil || err.Error() != "mock browse error" {
		t.Fatalf("expected 'mock browse error', got %v", err)
	}
}

func TestDiscover_Success(t *testing.T) {
	oldNewResolver := newResolver
	defer func() { newResolver = oldNewResolver }()
	newResolver = func() (mdnsResolver, error) {
		return &mockResolver{
			browseFunc: func(ctx context.Context, service string, domain string, entries chan<- *zeroconf.ServiceEntry) error {
				entries <- &zeroconf.ServiceEntry{
					Text: []string{"internal_url=http://ha.local:8123"},
				}
				return nil
			},
		}, nil
	}

	url, err := Discover(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if url != "http://ha.local:8123" {
		t.Errorf("expected 'http://ha.local:8123', got '%s'", url)
	}
}

func TestDiscover_ClosedChannelOrEmptyURL(t *testing.T) {
	oldNewResolver := newResolver
	defer func() { newResolver = oldNewResolver }()
	newResolver = func() (mdnsResolver, error) {
		return &mockResolver{
			browseFunc: func(ctx context.Context, service string, domain string, entries chan<- *zeroconf.ServiceEntry) error {
				entries <- &zeroconf.ServiceEntry{} // empty, should be ignored
				close(entries)
				return nil
			},
		}, nil
	}

	// Short timeout to avoid waiting 3 full seconds for the test to pass
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	url, err := Discover(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if url != "" {
		t.Errorf("expected empty url, got '%s'", url)
	}
}

func TestExtractURL(t *testing.T) {
	tests := []struct {
		name     string
		entry    *zeroconf.ServiceEntry
		expected string
	}{
		{
			name: "internal_url present",
			entry: &zeroconf.ServiceEntry{
				Text:     []string{"internal_url=http://ha.local:8123", "base_url=http://base.local"},
				AddrIPv4: []net.IP{net.ParseIP("192.168.1.100")},
				Port:     8123,
			},
			expected: "http://ha.local:8123",
		},
		{
			name: "base_url present",
			entry: &zeroconf.ServiceEntry{
				Text:     []string{"base_url=http://base.local:8123"},
				AddrIPv4: []net.IP{net.ParseIP("192.168.1.100")},
				Port:     8123,
			},
			expected: "http://base.local:8123",
		},
		{
			name: "empty txt records fallback to ip",
			entry: &zeroconf.ServiceEntry{
				Text:     []string{"internal_url=", "base_url="},
				AddrIPv4: []net.IP{net.ParseIP("192.168.1.100")},
				Port:     8123,
			},
			expected: "http://192.168.1.100:8123",
		},
		{
			name: "ignore https internal_url",
			entry: &zeroconf.ServiceEntry{
				Text:     []string{"internal_url=https://ha.local:8123", "base_url=http://base.local:8123"},
				AddrIPv4: []net.IP{net.ParseIP("192.168.1.100")},
				Port:     8123,
			},
			expected: "http://base.local:8123",
		},
		{
			name: "ignore https base_url fallback to ip",
			entry: &zeroconf.ServiceEntry{
				Text:     []string{"base_url=https://base.local:8123"},
				AddrIPv4: []net.IP{net.ParseIP("192.168.1.100")},
				Port:     8123,
			},
			expected: "http://192.168.1.100:8123",
		},
		{
			name: "no txt records fallback to ip",
			entry: &zeroconf.ServiceEntry{
				AddrIPv4: []net.IP{net.ParseIP("10.0.0.5")},
				Port:     8123,
			},
			expected: "http://10.0.0.5:8123",
		},
		{
			name:     "no useful info",
			entry:    &zeroconf.ServiceEntry{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := extractURL(tt.entry)
			if url != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, url)
			}
		})
	}
}
