package homeassistant

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
)

const homeAssistantService = "_home-assistant._tcp"

const discoveryTimeout = 5 * time.Second

var (
	netInterfaces = net.Interfaces
	mdnsQuery     = mdns.Query
)

type ServiceInstance struct {
	Name string
	URLs []string
}

func Discover(ctx context.Context) ([]ServiceInstance, error) {
	var ifaces []net.Interface
	if allIfaces, err := netInterfaces(); err == nil {
		for _, inf := range allIfaces {
			if inf.Flags&net.FlagUp != 0 && inf.Flags&net.FlagMulticast != 0 && inf.Flags&net.FlagLoopback == 0 {
				ifaces = append(ifaces, inf)
			}
		}
	}

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
				name = strings.TrimSuffix(name, "."+homeAssistantService+".local.")

				instances = append(instances, ServiceInstance{
					Name: name,
					URLs: instanceURLs,
				})
			}
		}
	}()

	// Query either all interfaces or specific ones
	if len(ifaces) == 0 {
		params := &mdns.QueryParam{
			Service:     homeAssistantService,
			Domain:      "local",
			Timeout:     discoveryTimeout,
			Entries:     entriesCh,
			DisableIPv6: true,
		}
		_ = mdnsQuery(params)
	} else {
		// Fire off queries for each interface in parallel to save time
		var queryWg sync.WaitGroup
		for _, inf := range ifaces {
			queryWg.Add(1)
			go func(iface net.Interface) {
				defer queryWg.Done()
				params := &mdns.QueryParam{
					Service:     homeAssistantService,
					Domain:      "local",
					Timeout:     discoveryTimeout,
					Entries:     entriesCh,
					Interface:   &iface,
					DisableIPv6: true,
				}
				_ = mdnsQuery(params)
			}(inf)
		}
		queryWg.Wait()
	}

	close(entriesCh)
	wg.Wait()

	return instances, nil
}

func isSupportedURL(url string) bool {
	return url != "" && (strings.HasPrefix(strings.ToLower(url), "http://") || strings.HasPrefix(strings.ToLower(url), "https://"))
}

func extractURLs(entry *mdns.ServiceEntry) []string {
	var urls []string

	// Check TXT records for configured URLs
	for _, txt := range entry.InfoFields {
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
	if entry.AddrV4 != nil {
		urls = append(urls, fmt.Sprintf("http://%s:%d", entry.AddrV4.String(), entry.Port))
	}

	return urls
}
