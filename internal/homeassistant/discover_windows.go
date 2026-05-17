//go:build windows

package homeassistant

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

func Discover(ctx context.Context) ([]ServiceInstance, error) {
	// 1. Create the mDNS query packet
	msg := new(dns.Msg)
	msg.SetQuestion(homeAssistantService, dns.TypePTR)
	msg.RecursionDesired = false
	buf, err := msg.Pack()
	if err != nil {
		return nil, fmt.Errorf("failed to pack mDNS query: %w", err)
	}

	// 2. Setup the listener on all multicast-capable interfaces
	addr, err := net.ResolveUDPAddr("udp4", "224.0.0.251:5353")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve multicast address: %w", err)
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list interfaces: %w", err)
	}

	resultsMu := sync.Mutex{}
	instancesMap := make(map[string]*ServiceInstance)

	var wg sync.WaitGroup
	
	// Create a context for the listener goroutines
	listenCtx, cancel := context.WithTimeout(ctx, discoveryTimeout)
	defer cancel()

	for _, inf := range ifaces {
		// Skip interfaces that are down or don't support multicast
		if inf.Flags&net.FlagMulticast == 0 || inf.Flags&net.FlagUp == 0 || inf.Flags&net.FlagLoopback != 0 {
			continue
		}

		wg.Add(1)
		go func(iface net.Interface) {
			defer wg.Done()
			
			conn, err := net.ListenMulticastUDP("udp4", &iface, addr)
			if err != nil {
				return
			}
			defer conn.Close()

			// Send the query on this interface
			_, _ = conn.WriteToUDP(buf, addr)

			// Listen for answers
			resBuf := make([]byte, 2048)
			for {
				select {
				case <-listenCtx.Done():
					return
				default:
					_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
					n, _, err := conn.ReadFromUDP(resBuf)
					if err != nil {
						if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
							continue
						}
						return
					}

					res := new(dns.Msg)
					if err := res.Unpack(resBuf[:n]); err != nil {
						continue
					}

					processResponse(res, &resultsMu, instancesMap)
				}
			}
		}(inf)
	}

	wg.Wait()

	var instances []ServiceInstance
	for _, inst := range instancesMap {
		instances = append(instances, *inst)
	}

	return instances, nil
}

func processResponse(res *dns.Msg, mu *sync.Mutex, instances map[string]*ServiceInstance) {
	mu.Lock()
	defer mu.Unlock()

	for _, answer := range res.Answer {
		ptr, ok := answer.(*dns.PTR)
		if !ok || !strings.Contains(ptr.Ptr, homeAssistantService) {
			continue
		}

		// Service name is like "Home._home-assistant._tcp.local."
		serviceInstanceName := ptr.Ptr
		friendlyName := strings.TrimSuffix(serviceInstanceName, "."+homeAssistantService)

		inst, exists := instances[serviceInstanceName]
		if !exists {
			inst = &ServiceInstance{Name: friendlyName}
			instances[serviceInstanceName] = inst
		}

		// Look for A, SRV, and TXT records in Answer and Extra sections
		allRRs := append(res.Answer, res.Extra...)
		for _, rr := range allRRs {
			switch v := rr.(type) {
			case *dns.A:
				url := fmt.Sprintf("http://%s:8123", v.A.String())
				if !contains(inst.URLs, url) {
					inst.URLs = append(inst.URLs, url)
				}
			case *dns.TXT:
				if strings.HasPrefix(v.Hdr.Name, serviceInstanceName) {
					for _, s := range v.Txt {
						if strings.HasPrefix(s, "internal_url=") || strings.HasPrefix(s, "base_url=") {
							url := strings.SplitN(s, "=", 2)[1]
							if isSupportedURL(url) && !contains(inst.URLs, url) {
								inst.URLs = append(inst.URLs, url)
							}
						}
					}
				}
			case *dns.SRV:
				if strings.HasPrefix(v.Hdr.Name, serviceInstanceName) {
					// Update port if we found an SRV record
					port := v.Port
					for i, url := range inst.URLs {
						if strings.Contains(url, ":8123") {
							inst.URLs[i] = strings.Replace(url, ":8123", fmt.Sprintf(":%d", port), 1)
						}
					}
				}
			}
		}
	}
}
