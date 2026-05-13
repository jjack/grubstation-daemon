package host

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
)

var (
	osHostname     = os.Hostname
	netInterfaces  = net.Interfaces
	netLookupCNAME = net.LookupCNAME
	getAddrs       = func(iface net.Interface) ([]net.Addr, error) {
		return iface.Addrs()
	}
	osStat = os.Stat
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

	return isPhysicalInterface(inf)
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
	ipv4 := ipnet.IP.To4()
	if ipv4 == nil {
		return nil // IPv6 doesn't use subnet broadcasts for WOL
	}

	mask := ipnet.Mask
	if len(mask) == 16 {
		// If it's a 16-byte mask for an IPv4 address, the mask is in the last 4 bytes.
		mask = mask[12:]
	}

	if len(mask) != 4 {
		return nil
	}

	last := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		last[i] = ipv4[i] | ^mask[i]
	}
	return last
}

// GetIPInfo returns a list of IP addresses (IPv4 and IPv6) and a map of those addresses to their computed broadcast address (for IPv4).
func GetIPInfo(inf net.Interface) ([]string, map[string]string) {
	var ips []string
	broadcasts := make(map[string]string)

	addrs, err := getAddrs(inf)
	if err != nil {
		return ips, broadcasts
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			ipv4 := ipnet.IP.To4()
			ip := ipv4
			if ipv4 == nil {
				ip = ipnet.IP // IPv6
			}

			ips = append(ips, ip.String())

			if broadcast := getLastIP(ipnet); broadcast != nil {
				broadcasts[ip.String()] = broadcast.String()
			}
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

// GetFQDN attempts to resolve the Fully Qualified Domain Name for a given hostname.
func GetFQDN(hostname string) string {
	if cname, err := netLookupCNAME(hostname); err == nil && cname != "" {
		return strings.TrimSuffix(cname, ".")
	}
	return hostname
}
