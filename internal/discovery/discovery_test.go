package discovery

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestLoopbackDNSSD(t *testing.T) {
	device := Device{Name: "Prüfgerät", SKI: "0123456789abcdef0123456789abcdef01234567", Brand: "Beispiel", Model: "CS", Address: "127.0.0.1:4711"}
	r, err := StartResponder(device)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, err := Search(ctx, r.Address())
	if err != nil || len(got) != 1 {
		t.Fatalf("%v %v", got, err)
	}
	if got[0].SKI != device.SKI || got[0].Brand != device.Brand || got[0].Model != device.Model || got[0].Address != device.Address {
		t.Fatalf("wrong records: %+v", got)
	}
	if _, err := Search(ctx, "192.0.2.1:5353"); err == nil {
		t.Fatal("LAN query allowed")
	}
}

func TestLANRequiresExplicitInterface(t *testing.T) {
	for _, name := range []string{"", "no-such-stromea-interface"} {
		if _, err := SearchLAN(context.Background(), name); err == nil {
			t.Fatalf("interface %q accepted", name)
		}
	}
}

// Synthetic LAN records only: this test never sends a packet to a LAN interface.
func TestLANRecordAssemblyAndBoundary(t *testing.T) {
	_, subnet, _ := net.ParseCIDR("192.0.2.0/24")
	allow := func(ip net.IP) bool { return subnet.Contains(ip) }
	d := Device{Name: "Vitocal", SKI: "0123456789abcdef0123456789abcdef01234567", Address: "192.0.2.42:4711"}
	m := records(d)
	cache := newRecordCache()
	cache.add(&dns.Msg{Answer: m.Answer})
	if got := parseAllowed(cache.message(), allow); len(got) != 0 {
		t.Fatal("incomplete service accepted", got)
	}
	cache.add(&dns.Msg{Extra: m.Extra[:2]})
	cache.add(&dns.Msg{Answer: m.Extra[2:]})
	got := parseAllowed(cache.message(), allow)
	if len(got) != 1 || got[0].Address != d.Address || got[0].SKI != d.SKI {
		t.Fatalf("split DNS-SD: %+v", got)
	}
	if got := parse(cache.message()); len(got) != 0 {
		t.Fatal("default discovery accepted LAN", got)
	}
	m.Extra[2].(*dns.A).A = net.ParseIP("198.51.100.42")
	if got := parseAllowed(m, allow); len(got) != 0 {
		t.Fatal("off-interface address accepted", got)
	}
	m.Extra[2].(*dns.A).A = net.ParseIP("192.0.2.42")
	m.Extra[0].(*dns.SRV).Port = 0
	if got := parseAllowed(m, allow); len(got) != 0 {
		t.Fatal("zero port accepted", got)
	}
	m.Answer[0].Header().Ttl = 0
	cache.add(&dns.Msg{Answer: m.Answer})
	if got := parseAllowed(cache.message(), allow); len(got) != 0 {
		t.Fatal("goodbye service retained", got)
	}
}

func TestLANInterfaceValidation(t *testing.T) {
	for _, flags := range []net.Flags{0, net.FlagUp, net.FlagUp | net.FlagMulticast | net.FlagLoopback} {
		if _, _, err := interfaceNetwork(net.Interface{Flags: flags}, []net.Addr{&net.IPNet{IP: net.IPv4(192, 0, 2, 1), Mask: net.CIDRMask(24, 32)}}); err == nil {
			t.Fatal("unsafe interface accepted", flags)
		}
	}
	iface := net.Interface{Flags: net.FlagUp | net.FlagMulticast}
	ip, subnet, err := interfaceNetwork(iface, []net.Addr{&net.IPNet{IP: net.IPv4(192, 0, 2, 1), Mask: net.CIDRMask(24, 32)}})
	if err != nil || !ip.Equal(net.ParseIP("192.0.2.1")) || !subnet.Contains(net.ParseIP("192.0.2.42")) {
		t.Fatalf("%v %v %v", ip, subnet, err)
	}
}

func TestMDNSFollowupAndExpiry(t *testing.T) {
	d := Device{SKI: "0123456789abcdef0123456789abcdef01234567", Address: "127.0.0.1:4711"}
	cache := newRecordCache()
	m := records(d)
	cache.add(m)
	if qs := cache.questions(); len(qs) != 3 || qs[0].Qclass&0x8000 == 0 {
		t.Fatalf("missing QU follow-up questions: %+v", qs)
	}
	for k, v := range cache.records {
		v.expires = time.Now().Add(-time.Second)
		cache.records[k] = v
	}
	if len(cache.message().Answer) != 0 {
		t.Fatal("expired records retained")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := SearchLAN(ctx, "unused-interface"); err != context.Canceled {
		t.Fatalf("cancelled LAN search: %v", err)
	}
}
