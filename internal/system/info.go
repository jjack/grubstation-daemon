package system

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
)

var (
	osHostname    = os.Hostname
	netInterfaces = net.Interfaces
	getAddrs      = func(iface net.Interface) ([]net.Addr, error) {
		return iface.Addrs()
	}
)

var (
	ErrListInterfaces       = errors.New("failed to list network interfaces")
	ErrNoSuitableInterfaces = errors.New("no suitable interfaces found")
	ErrDetectHostname       = errors.New("failed to detect hostname")
)

type InterfaceInfo struct {
	Label string
	Value string
}

// GetIPAddrs returns all addresses for a given interface as strings.
func GetIPAddrs(iface net.Interface) []string {
	addrs, err := getAddrs(iface)
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
	interfaces, err := netInterfaces()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrListInterfaces, err)
	}

	var options []InterfaceInfo
	for _, inf := range interfaces {
		if len(inf.HardwareAddr) == 0 || inf.Flags&net.FlagUp == 0 || inf.Flags&net.FlagLoopback != 0 {
			continue
		}

		macStr := inf.HardwareAddr.String()
		ips := strings.Join(GetIPAddrs(inf), ", ")
		label := fmt.Sprintf("%s (%s) [%s]", inf.Name, macStr, ips)
		options = append(options, InterfaceInfo{Label: label, Value: macStr})
	}

	if len(options) == 0 {
		return nil, ErrNoSuitableInterfaces
	}

	return options, nil
}

func DetectHostname() (string, error) {
	hostname, err := osHostname()
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrDetectHostname, err)
	}
	return hostname, nil
}

func GetBroadcastAddresses(mac string) ([]string, error) {
	interfaces, err := netInterfaces()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrListInterfaces, err)
	}

	var broadcasts []string
	seen := make(map[string]bool)
	for _, inf := range interfaces {
		if strings.EqualFold(inf.HardwareAddr.String(), mac) {
			addrs, err := getAddrs(inf)
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok {
					ip := ipnet.IP.To4()
					if ip != nil {
						mask := ipnet.Mask
						if len(mask) == net.IPv6len {
							mask = mask[12:]
						}
						if len(mask) == net.IPv4len {
							broadcast := net.IPv4(
								ip[0]|^mask[0],
								ip[1]|^mask[1],
								ip[2]|^mask[2],
								ip[3]|^mask[3],
							).String()
							if !seen[broadcast] {
								broadcasts = append(broadcasts, broadcast)
								seen[broadcast] = true
							}
						}
					}
				}
			}
			break
		}
	}
	if len(broadcasts) == 0 {
		return []string{"255.255.255.255"}, nil
	}
	return broadcasts, nil
}
