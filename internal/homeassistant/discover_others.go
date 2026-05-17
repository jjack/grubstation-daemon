//go:build !windows

package homeassistant

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/hashicorp/mdns"
)

var (
	mdnsQuery = mdns.Query
)

func Discover(ctx context.Context) ([]ServiceInstance, error) {
	// hashicorp/mdns uses a channel for results
	entriesCh := make(chan *mdns.ServiceEntry, 50)
	var instances []ServiceInstance
	seen := make(map[string]bool)

	// Start a goroutine to collect results
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for entry := range entriesCh {
			var instanceURLs []string
			for _, url := range extractURLs(entry) {
				if url != "" && !seen[url] {
					seen[url] = true
					instanceURLs = append(instanceURLs, url)
				}
			}

			if len(instanceURLs) > 0 {
				name := entry.Name
				if name == "" {
					name = "Home Assistant"
				}
				// Clean up the name if it has the service suffix
				name = strings.TrimSuffix(name, "."+homeAssistantService)

				instances = append(instances, ServiceInstance{
					Name: name,
					URLs: instanceURLs,
				})
			}
		}
	}()

	// Quiet hashicorp/mdns's spammy logging
	silentLogger := log.New(io.Discard, "", 0)

	params := &mdns.QueryParam{
		Service:     strings.TrimSuffix(homeAssistantService, "."),
		Domain:      "local",
		Timeout:     discoveryTimeout * 2,
		Entries:     entriesCh,
		DisableIPv6: false,
		Logger:      silentLogger,
	}

	// First, try a global query
	_ = mdnsQuery(params)

	// If we found nothing, try specific interfaces as a fallback
	if len(instances) == 0 {
		if ifaces, err := netInterfaces(); err == nil {
			var queryWg sync.WaitGroup
			for _, inf := range ifaces {
				if inf.Flags&net.FlagUp != 0 && inf.Flags&net.FlagMulticast != 0 && inf.Flags&net.FlagLoopback == 0 {
					queryWg.Add(1)
					go func(iface net.Interface) {
						defer queryWg.Done()
						ifaceParams := *params
						ifaceParams.Interface = &iface
						_ = mdnsQuery(&ifaceParams)
					}(inf)
				}
			}
			queryWg.Wait()
		}
	}

	close(entriesCh)
	wg.Wait()

	return instances, nil
}

func extractURLs(entry *mdns.ServiceEntry) []string {
	var urls []string
	for _, txt := range entry.InfoFields {
		if strings.HasPrefix(txt, "internal_url=") || strings.HasPrefix(txt, "base_url=") {
			url := strings.SplitN(txt, "=", 2)[1]
			if isSupportedURL(url) {
				urls = append(urls, url)
			}
		}
	}
	if entry.AddrV4 != nil {
		urls = append(urls, fmt.Sprintf("http://%s:%d", entry.AddrV4.String(), entry.Port))
	}
	return urls
}
