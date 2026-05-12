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

type ServiceInstance struct {
	Name string
	URLs []string
}

func Discover(ctx context.Context) ([]ServiceInstance, error) {
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

	var instances []ServiceInstance
	seen := make(map[string]bool)

	for {
		select {
		case err, ok := <-errChan:
			if !ok {
				continue
			}
			if err != nil {
				return nil, err
			}
			errChan = nil
		case entry, ok := <-entries:
			if !ok {
				return instances, nil
			}

			var instanceURLs []string
			for _, url := range extractURLs(entry) {
				if url != "" && !seen[url] {
					seen[url] = true
					instanceURLs = append(instanceURLs, url)
				}
			}

			if len(instanceURLs) > 0 {
				name := entry.Instance
				if name == "" {
					name = "Home Assistant"
				}
				instances = append(instances, ServiceInstance{
					Name: name,
					URLs: instanceURLs,
				})
			}
		}
	}
}

func isSupportedURL(url string) bool {
	return url != "" && (strings.HasPrefix(strings.ToLower(url), "http://") || strings.HasPrefix(strings.ToLower(url), "https://"))
}

func extractURLs(entry *zeroconf.ServiceEntry) []string {
	var urls []string

	// Check TXT records for configured URLs
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

	// Add all IPv4 addresses as potential URLs
	for _, ip := range entry.AddrIPv4 {
		urls = append(urls, fmt.Sprintf("http://%s:%d", ip.String(), entry.Port))
	}

	return urls
}
