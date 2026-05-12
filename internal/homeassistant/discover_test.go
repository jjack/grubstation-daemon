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
	urls, err := Discover(context.Background())
	if err != nil {
		t.Fatalf("expected no error on timeout, got %v", err)
	}
	if len(urls) > 0 {
		t.Logf("Found HA at %v", urls)
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
					AddrIPv4: []net.IP{net.ParseIP("192.168.1.100")},
					Port:     8123,
					Text:     []string{"internal_url=http://ha.local:8123"},
				}
				return nil
			},
		}, nil
	}

	urls, err := Discover(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(urls) != 2 || urls[0] != "http://192.168.1.100:8123" || urls[1] != "http://ha.local:8123" {
		t.Errorf("expected ['http://192.168.1.100:8123', 'http://ha.local:8123'], got %v", urls)
	}
}

func TestDiscover_MultipleSuccess(t *testing.T) {
	oldNewResolver := newResolver
	defer func() { newResolver = oldNewResolver }()
	newResolver = func() (mdnsResolver, error) {
		return &mockResolver{
			browseFunc: func(ctx context.Context, service string, domain string, entries chan<- *zeroconf.ServiceEntry) error {
				entries <- &zeroconf.ServiceEntry{
					AddrIPv4: []net.IP{net.ParseIP("192.168.1.100")},
					Port:     8123,
					Text:     []string{"internal_url=http://ha1.local:8123"},
				}
				entries <- &zeroconf.ServiceEntry{
					AddrIPv4: []net.IP{net.ParseIP("192.168.1.101")},
					Port:     8123,
					Text:     []string{"internal_url=http://ha2.local:8123"},
				}
				entries <- &zeroconf.ServiceEntry{
					AddrIPv4: []net.IP{net.ParseIP("192.168.1.100")}, // Duplicate IP
					Port:     8123,
					Text:     []string{"internal_url=http://ha1.local:8123"}, // Duplicate URL
				}
				return nil
			},
		}, nil
	}

	urls, err := Discover(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(urls) != 4 {
		t.Errorf("expected 4 unique URLs, got %d: %v", len(urls), urls)
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
		entry    *zeroconf.ServiceEntry
		expected []string
	}{
		{
			name: "internal_url and ip present",
			entry: &zeroconf.ServiceEntry{
				Text:     []string{"internal_url=http://ha.local:8123", "base_url=http://base.local"},
				AddrIPv4: []net.IP{net.ParseIP("192.168.1.100")},
				Port:     8123,
			},
			expected: []string{"http://192.168.1.100:8123", "http://ha.local:8123", "http://base.local"},
		},
		{
			name: "base_url present",
			entry: &zeroconf.ServiceEntry{
				Text:     []string{"base_url=http://base.local:8123"},
				AddrIPv4: []net.IP{net.ParseIP("192.168.1.100")},
				Port:     8123,
			},
			expected: []string{"http://192.168.1.100:8123", "http://base.local:8123"},
		},
		{
			name: "empty txt records only ip",
			entry: &zeroconf.ServiceEntry{
				Text:     []string{"internal_url=", "base_url="},
				AddrIPv4: []net.IP{net.ParseIP("192.168.1.100")},
				Port:     8123,
			},
			expected: []string{"http://192.168.1.100:8123"},
		},
		{
			name: "ignore https internal_url",
			entry: &zeroconf.ServiceEntry{
				Text:     []string{"internal_url=https://ha.local:8123", "base_url=http://base.local:8123"},
				AddrIPv4: []net.IP{net.ParseIP("192.168.1.100")},
				Port:     8123,
			},
			expected: []string{"http://192.168.1.100:8123", "http://base.local:8123"},
		},
		{
			name: "ignore https base_url",
			entry: &zeroconf.ServiceEntry{
				Text:     []string{"base_url=https://base.local:8123"},
				AddrIPv4: []net.IP{net.ParseIP("192.168.1.100")},
				Port:     8123,
			},
			expected: []string{"http://192.168.1.100:8123"},
		},
		{
			name: "no txt records only ip",
			entry: &zeroconf.ServiceEntry{
				AddrIPv4: []net.IP{net.ParseIP("10.0.0.5")},
				Port:     8123,
			},
			expected: []string{"http://10.0.0.5:8123"},
		},
		{
			name:     "no useful info",
			entry:    &zeroconf.ServiceEntry{},
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
