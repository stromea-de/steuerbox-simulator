package discovery

import (
	"context"
	"github.com/miekg/dns"
	"net"
	"testing"
	"time"
)

// First UDP query is deliberately lost. A single search must retry on its own.
func TestSingleSearchRetriesLostFirstQuery024(t *testing.T) {
	server, e := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if e != nil {
		t.Fatal(e)
	}
	defer server.Close()
	done := make(chan struct{})
	defer func() { server.Close(); <-done }()
	go func() {
		defer close(done)
		b := make([]byte, 9000)
		queries := 0
		for {
			n, peer, e := server.ReadFromUDP(b)
			if e != nil {
				return
			}
			var q dns.Msg
			if q.Unpack(b[:n]) != nil {
				continue
			}
			queries++
			if queries == 1 {
				continue
			}
			m := records(Device{SKI: "0123456789abcdef0123456789abcdef01234567", Address: "127.0.0.1:4711"})
			m.Response = true
			m.Id = q.Id
			packet, _ := m.Pack()
			server.WriteToUDP(packet, peer)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()
	got, e := Search(ctx, server.LocalAddr().String())
	if e != nil || len(got) != 1 {
		t.Fatalf("first click: %v %v", got, e)
	}
}

func TestLiveSplitResponsesAndDelayedAddress024(t *testing.T) {
	server, e := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if e != nil {
		t.Fatal(e)
	}
	defer server.Close()
	client, e := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if e != nil {
		t.Fatal(e)
	}
	defer client.Close()
	d := Device{SKI: "0123456789abcdef0123456789abcdef01234567", Address: "127.0.0.1:4711"}
	ctx, cancel := context.WithTimeout(context.Background(), 3200*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 9000)
		_, peer, e := server.ReadFromUDP(buf)
		if e != nil {
			return
		}
		m := records(d)
		reply := &dns.Msg{MsgHdr: dns.MsgHdr{Response: true}, Answer: m.Answer, Extra: m.Extra[:2]}
		b, _ := reply.Pack()
		server.WriteToUDP(b, peer)
		timer := time.NewTimer(2200 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		reply.Extra = m.Extra[2:]
		b, _ = reply.Pack()
		server.WriteToUDP(b, peer)
	}()
	updates := 0
	observed := false
	got, e := exchangeAllowed(ctx, client, server.LocalAddr().(*net.UDPAddr), true, func(ip net.IP) bool { return ip.IsLoopback() }, func(v []Device) {
		updates++
		if len(v) > 0 {
			observed = true
			if ctx.Err() != nil {
				t.Error("live result arrived after deadline")
			}
		}
	})
	<-done
	if len(got) != 1 || !observed || updates < 2 || (e != nil && e != context.DeadlineExceeded) {
		t.Fatal(got, e, observed, updates)
	}
}
