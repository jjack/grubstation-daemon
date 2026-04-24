package homeassistant

import (
	"context"
	"fmt"
	"time"

	"github.com/grandcat/zeroconf"
)

const (
	homeAssistantService    = "_home-assistant._tcp"
	homeAssistantDefaultURL = "http://homeassistant.local:8123"
)

const discoveryTimeout = 3 * time.Second

func Discover() (string, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return "", err
	}

	entries := make(chan *zeroconf.ServiceEntry)
	found := make(chan string, 1) // Channel to receive the discovered URL

	ctx, cancel := context.WithTimeout(context.Background(), discoveryTimeout)
	defer cancel()

	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			// Found it! Grab the first IPv4 address
			if len(entry.AddrIPv4) > 0 {
				ip := entry.AddrIPv4[0].String()
				port := entry.Port
				url := fmt.Sprintf("http://%s:%d", ip, port)
				found <- url
				cancel() // Cancel context to stop spinner and discovery early
				return
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
		return homeAssistantDefaultURL, nil
	}
}
