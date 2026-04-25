package system

import (
	"fmt"
	"net"
	"os"
)

type InterfaceInfo struct {
	Label string
	Value string
}

// GetIPAddrs returns all addresses for a given interface as strings.
func GetIPAddrs(iface net.Interface) []string {
	addrs, err := iface.Addrs()
	if err != nil {
		return nil
	}
	var ipAddrs []string
	for _, addr := range addrs {
		ipAddrs = append(ipAddrs, addr.String())
	}
	return ipAddrs
}

// GetInterfaceOptions returns a slice of label/value pairs for use in selection UIs.
func GetInterfaceOptions() ([]InterfaceInfo, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list network interfaces: %w", err)
	}

	var options []InterfaceInfo
	for _, inf := range interfaces {
		if len(inf.HardwareAddr) == 0 || inf.Flags&net.FlagUp == 0 || inf.Flags&net.FlagLoopback != 0 {
			continue
		}

		macStr := inf.HardwareAddr.String()
		label := fmt.Sprintf("%s (%s) [%v]", inf.Name, macStr, GetIPAddrs(inf))
		options = append(options, InterfaceInfo{Label: label, Value: macStr})
	}

	if len(options) == 0 {
		return nil, fmt.Errorf("no suitable interfaces found")
	}

	return options, nil
}

func DetectHostname() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("failed to detect hostname: %w", err)
	}
	return hostname, nil
}
