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

// isWOLCapableInterface checks if the given network interface is suitable for WOL (has a MAC address, is up, is not loopback, and is not virtual).
func isWOLCapableInterface(inf net.Interface) bool {
	if len(inf.HardwareAddr) == 0 {
		return false
	}

	if inf.Flags&net.FlagUp == 0 {
		return false
	}

	if inf.Flags&net.FlagLoopback != 0 {
		return false
	}

	virtualInterfaces := []string{"veth", "docker", "br-", "virbr", "vmnet", "vboxnet"}
	for _, prefix := range virtualInterfaces {
		if strings.HasPrefix(inf.Name, prefix) {
			return false
		}
	}

	path := fmt.Sprintf("/sys/class/net/%s/device", inf.Name)
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// GetWOLInterfaces returns a slice of net.Interface that are capable of Wake-on-LAN.
func GetWOLInterfaces() ([]net.Interface, error) {
	interfaces, err := netInterfaces()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrListInterfaces, err)
	}

	var wolIfaces []net.Interface
	for _, inf := range interfaces {
		if isWOLCapableInterface(inf) {
			wolIfaces = append(wolIfaces, inf)
		}
	}

	if len(wolIfaces) == 0 {
		return nil, ErrNoSuitableInterfaces
	}

	return wolIfaces, nil
}

func getLastIP(ipnet *net.IPNet) net.IP {
	// The net.IP mask is a byte slice. To get the last IP,
	// we OR the IP with the inverse of the mask.
	ip := ipnet.IP.To4()
	if ip == nil {
		ip = ipnet.IP // IPv6
	}

	last := make(net.IP, len(ip))
	for i := 0; i < len(ip); i++ {
		last[i] = ip[i] | ^ipnet.Mask[i]
	}
	return last
}

// GetIPv4Info returns a list of IPv4 addresses and a map of those addresses to their computed broadcast address.
func GetIPv4Info(inf net.Interface) ([]string, map[string]string) {
	var ips []string
	broadcasts := make(map[string]string)

	addrs, err := getAddrs(inf)
	if err != nil {
		return ips, broadcasts
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			ip := ipnet.IP.To4()
			if ip == nil {
				ip = ipnet.IP // IPv6
			}

			ips = append(ips, ip.String())

			broadcast := getLastIP(ipnet)
			broadcasts[ip.String()] = broadcast.String()
		}
	}
	return ips, broadcasts
}

func DetectHostname() (string, error) {
	hostname, err := osHostname()
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrDetectHostname, err)
	}
	return hostname, nil
}
