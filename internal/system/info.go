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

// DetectUsablenterfaces returns all usable network interfaces (non-loopback, up, with MAC).
func DetectUsablenterfaces() ([]net.Interface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list network interfaces: %w", err)
	}

	usable := []net.Interface{}
	for _, interf := range interfaces {
		if len(interf.HardwareAddr) > 0 && interf.Flags&net.FlagUp != 0 && interf.Flags&net.FlagLoopback == 0 {
			usable = append(usable, interf)
		}
	}
	if len(usable) == 0 {
		return nil, fmt.Errorf("no suitable interfaces found")
	}
	return usable, nil
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
	interfaces, err := DetectUsablenterfaces()
	if err != nil {
		return nil, err
	}

	options := make([]InterfaceInfo, len(interfaces))
	for i, inf := range interfaces {
		addr, err := net.ParseMAC(inf.HardwareAddr.String())
		if err != nil {
			return nil, fmt.Errorf("failed to parse MAC address: %w", err)
		}

		label := fmt.Sprintf("%s (%s) [%v]", inf.Name, addr.String(), GetIPAddrs(inf))
		options[i] = InterfaceInfo{Label: label, Value: addr.String()}
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
