package homeassistant

import (
	"net"
	"strings"
	"time"
)

const homeAssistantService = "_home-assistant._tcp.local."

var (
	discoveryTimeout = 5 * time.Second
	netInterfaces    = net.Interfaces
)

type ServiceInstance struct {
	Name string
	URLs []string
}

func isSupportedURL(url string) bool {
	return url != "" && (strings.HasPrefix(strings.ToLower(url), "http://") || strings.HasPrefix(strings.ToLower(url), "https://"))
}

func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
