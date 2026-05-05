package homeassistant

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
)

const homeAssistantService = "_home-assistant._tcp"

const discoveryTimeout = 3 * time.Second

type mdnsResolver interface {
	Browse(ctx context.Context, service string, domain string, entries chan<- *zeroconf.ServiceEntry) error
}

var newResolver = func() (mdnsResolver, error) {
	return zeroconf.NewResolver(nil)
}

func Discover(ctx context.Context) (string, error) {
	resolver, err := newResolver()
	if err != nil {
		return "", err
	}

	entries := make(chan *zeroconf.ServiceEntry)
	found := make(chan string, 1) // Channel to receive the discovered URL

	ctx, cancel := context.WithTimeout(ctx, discoveryTimeout)
	defer cancel()

	go func(results <-chan *zeroconf.ServiceEntry) {
		for {
			select {
			case <-ctx.Done():
				return
			case entry, ok := <-results:
				if !ok {
					return
				}

				url := extractURL(entry)
				if url != "" {
					found <- url
					return
				}
			}
		}
	}(entries)

	err = resolver.Browse(ctx, homeAssistantService, "local.", entries)
	if err != nil {
		return "", err
	}

	select {
	case url := <-found:
		return url, nil
	case <-ctx.Done():
		return "", nil
	}
}

func extractURL(entry *zeroconf.ServiceEntry) string {
	// Check TXT records for configured URLs first
	for _, txt := range entry.Text {
		if strings.HasPrefix(txt, "internal_url=") {
			if url := strings.TrimPrefix(txt, "internal_url="); url != "" {
				return url
			}
		}
		if strings.HasPrefix(txt, "base_url=") {
			if url := strings.TrimPrefix(txt, "base_url="); url != "" {
				return url
			}
		}
	}

	// Fall back to constructing URL from IP and port if no suitable TXT record is found
	if len(entry.AddrIPv4) > 0 {
		ip := entry.AddrIPv4[0].String()
		port := entry.Port
		return fmt.Sprintf("http://%s:%d", ip, port)
	}
	return ""
}
