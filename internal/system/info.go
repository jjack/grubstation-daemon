package system

import (
	"fmt"
	"net"
	"os"
)

func DetectMacAddress() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("failed to list network interfaces: %w", err)
	}

	for _, interf := range interfaces {
		if interf.HardwareAddr != nil && len(interf.HardwareAddr) > 0 {
			if interf.Flags&net.FlagUp != 0 && interf.Flags&net.FlagLoopback == 0 {
				return interf.HardwareAddr.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no suitable MAC address found")
}

func DetectHostname() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("failed to detect hostname: %w", err)
	}
	return hostname, nil
}
