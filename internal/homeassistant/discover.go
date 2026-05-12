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

func Discover(ctx context.Context) ([]string, error) {
	resolver, err := newResolver()
	if err != nil {
		return nil, err
	}

	entries := make(chan *zeroconf.ServiceEntry)

	ctx, cancel := context.WithTimeout(ctx, discoveryTimeout)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- resolver.Browse(ctx, homeAssistantService, "local.", entries)
	}()

	var urls []string
	seen := make(map[string]bool)

	for {
		select {
		case err := <-errChan:
			if err != nil {
				return nil, err
			}
			return urls, nil
		case <-ctx.Done():
			return urls, nil
		case entry, ok := <-entries:
			if !ok {
				continue
			}

			for _, url := range extractURLs(entry) {
				if url != "" && !seen[url] {
					seen[url] = true
					urls = append(urls, url)
				}
			}
		}
	}
}

func isSupportedURL(url string) bool {
	return url != "" && !strings.HasPrefix(strings.ToLower(url), "https://")
}

func extractURLs(entry *zeroconf.ServiceEntry) []string {
	var urls []string

	if len(entry.AddrIPv4) > 0 {
		ip := entry.AddrIPv4[0].String()
		port := entry.Port
		urls = append(urls, fmt.Sprintf("http://%s:%d", ip, port))
	}

	for _, txt := range entry.Text {
		if strings.HasPrefix(txt, "internal_url=") {
			if url := strings.TrimPrefix(txt, "internal_url="); isSupportedURL(url) {
				urls = append(urls, url)
			}
		}
		if strings.HasPrefix(txt, "base_url=") {
			if url := strings.TrimPrefix(txt, "base_url="); isSupportedURL(url) {
				urls = append(urls, url)
			}
		}
	}

	return urls
}
