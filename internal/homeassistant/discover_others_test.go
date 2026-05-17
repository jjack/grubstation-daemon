//go:build windows

package homeassistant

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestDiscover_Timeout(t *testing.T) {
	// Set a very short timeout for the test
	oldTimeout := discoveryTimeout
	discoveryTimeout = 10 * time.Millisecond
	defer func() { discoveryTimeout = oldTimeout }()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := Discover(ctx)
	if err != nil {
		t.Fatalf("expected no error on timeout, got %v", err)
	}
}

func TestDiscover_Success(t *testing.T) {
	// Set a short timeout for the test
	oldTimeout := discoveryTimeout
	discoveryTimeout = 100 * time.Millisecond
	defer func() { discoveryTimeout = oldTimeout }()

	addr, _ := net.ResolveUDPAddr("udp4", "224.0.0.251:5353")
	
	stop := make(chan struct{})
	go func() {
		conn, err := net.ListenMulticastUDP("udp4", nil, addr)
		if err != nil {
			return
		}
		defer conn.Close()

		go func() {
			b := make([]byte, 2048)
			for {
				n, src, err := conn.ReadFromUDP(b)
				if err != nil {
					return
				}
				if n > 0 {
					msg := new(dns.Msg)
					if err := msg.Unpack(b[:n]); err == nil {
						if len(msg.Question) > 0 && strings.Contains(msg.Question[0].Name, homeAssistantService) {
							res := new(dns.Msg)
							res.MsgHdr.Response = true
							res.MsgHdr.Authoritative = true
							
							ptr := &dns.PTR{
								Hdr: dns.RR_Header{Name: homeAssistantService, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: 120},
								Ptr: "MockHA." + homeAssistantService,
							}
							res.Answer = append(res.Answer, ptr)
							
							txt := &dns.TXT{
								Hdr: dns.RR_Header{Name: "MockHA." + homeAssistantService, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 120},
								Txt: []string{"internal_url=http://mockha.local:8123"},
							}
							res.Extra = append(res.Extra, txt)
							
							a := &dns.A{
								Hdr: dns.RR_Header{Name: "mockha-uuid.local.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 120},
								A:   net.ParseIP("127.0.0.1"),
							}
							res.Extra = append(res.Extra, a)
							
							srv := &dns.SRV{
								Hdr:    dns.RR_Header{Name: "MockHA." + homeAssistantService, Rrtype: dns.TypeSRV, Class: dns.ClassINET, Ttl: 120},
								Target: "mockha-uuid.local.",
								Port:   8123,
							}
							res.Extra = append(res.Extra, srv)

							out, _ := res.Pack()
							_, _ = conn.WriteToUDP(out, src)
						}
					}
				}
			}
		}()
		<-stop
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	instances, err := Discover(ctx)
	close(stop)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(instances) == 0 {
		t.Skip("No instances found - multicast loopback might be disabled")
	}

	found := false
	for _, inst := range instances {
		if inst.Name == "MockHA" {
			found = true
			urls := inst.URLs
			if !contains(urls, "http://mockha.local:8123") || !contains(urls, "http://127.0.0.1:8123") {
				t.Errorf("expected URLs not found in %v", urls)
			}
		}
	}
}
