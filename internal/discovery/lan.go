package discovery

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/net/ipv4"
)

type Interface struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Kind    string `json:"kind"`
}

// Interfaces enumerates eligible IPv4 multicast interfaces without sending packets.
func Interfaces() ([]Interface, error) {
	all, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	records := []interfaceRecord{}
	for _, iface := range all {
		addrs, err := iface.Addrs()
		if err == nil {
			records = append(records, interfaceRecord{iface, addrs})
		}
	}
	return availableInterfaces(records, platformInterfaceKinds(all)), nil
}

func interfaceNetwork(iface net.Interface, addrs []net.Addr) (net.IP, *net.IPNet, error) {
	if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 || iface.Flags&net.FlagLoopback != 0 {
		return nil, nil, errors.New("aktives Multicast-LAN-Interface erforderlich")
	}
	var best net.IP
	var subnet *net.IPNet
	for _, addr := range addrs {
		ip, network, err := net.ParseCIDR(addr.String())
		if err == nil && ip.To4() != nil && !ip.IsLoopback() && !ip.IsUnspecified() && !ip.IsMulticast() {
			if best == nil || addressRank(ip) < addressRank(best) {
				best, subnet = ip.To4(), network
			}
		}
	}
	if best != nil {
		return best, subnet, nil
	}

	return nil, nil, errors.New("Interface hat keine nutzbare IPv4-Adresse")
}

// SearchLAN performs only DNS-SD reads. It creates no SHIP session, identity,
// trust entry or SPINE write. Calling it is the explicit network opt-in.
// The preferred IPv4 subnet of the named interface is used; no default route or DNS.
func SearchLAN(ctx context.Context, name string) ([]Device, error) {
	return SearchLANLive(ctx, name, nil)
}

func SearchLANLive(ctx context.Context, name string, update func([]Device)) ([]Device, error) {
	if name == "" {
		return nil, errors.New("--lan erfordert --interface")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, fmt.Errorf("Interface: %w", err)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	ip, subnet, err := interfaceNetwork(*iface, addrs)
	if err != nil {
		return nil, err
	}
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: ip})
	if err != nil {
		return nil, err
	}
	defer c.Close()
	p := ipv4.NewPacketConn(c)
	if err = p.SetMulticastInterface(iface); err != nil {
		return nil, err
	}
	if err = p.SetMulticastTTL(255); err != nil {
		return nil, err
	}
	if err = p.SetMulticastLoopback(false); err != nil {
		return nil, err
	}
	annotate := func(v []Device) {
		if update != nil {
			update(withRoute(v, name, ip.String()))
		}
	}
	found, err := exchangeAllowed(ctx, c, &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}, true, func(peer net.IP) bool {
		return peer.To4() != nil && !peer.IsLoopback() && !peer.IsMulticast() && !peer.IsUnspecified() && subnet.Contains(peer)
	}, annotate)
	return withRoute(found, name, ip.String()), err
}

// Small, per-search cache handles split replies, TTL expiry and goodbye packets.
// A hard cap bounds memory and follow-up traffic from unauthenticated mDNS data.
type cachedRecord struct {
	rr      dns.RR
	expires time.Time
}
type recordCache struct{ records map[string]cachedRecord }

func newRecordCache() *recordCache { return &recordCache{records: map[string]cachedRecord{}} }
func recordKey(rr dns.RR) string {
	c := dns.Copy(rr)
	c.Header().Ttl = 0
	c.Header().Class &^= 1 << 15
	return strings.ToLower(c.String())
}
func (c *recordCache) add(m *dns.Msg) {
	for _, rr := range append(append([]dns.RR{}, m.Answer...), m.Extra...) {
		if rr.Header().Class&0x7fff != dns.ClassINET {
			continue
		}
		key := recordKey(rr)
		if rr.Header().Ttl == 0 {
			delete(c.records, key)
			continue
		}
		if len(c.records) >= 256 {
			continue
		}
		c.records[key] = cachedRecord{dns.Copy(rr), time.Now().Add(time.Duration(rr.Header().Ttl) * time.Second)}
	}
}
func (c *recordCache) message() *dns.Msg {
	m := new(dns.Msg)
	keys := make([]string, 0, len(c.records))
	for k := range c.records {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		v := c.records[key]
		if time.Now().After(v.expires) {
			delete(c.records, key)
			continue
		}
		m.Answer = append(m.Answer, v.rr)
	}
	return m
}
func (c *recordCache) questions() []dns.Question {
	out := []dns.Question{}
	for _, rr := range c.message().Answer {
		switch v := rr.(type) {
		case *dns.PTR:
			if strings.EqualFold(v.Hdr.Name, Service) && strings.HasSuffix(strings.ToLower(v.Ptr), "."+Service) {
				out = append(out, dns.Question{Name: v.Ptr, Qtype: dns.TypeSRV, Qclass: dns.ClassINET | 1<<15}, dns.Question{Name: v.Ptr, Qtype: dns.TypeTXT, Qclass: dns.ClassINET | 1<<15})
			}
		case *dns.SRV:
			if strings.HasSuffix(strings.ToLower(v.Hdr.Name), "."+Service) && strings.HasSuffix(strings.ToLower(v.Target), ".local.") {
				out = append(out, dns.Question{Name: v.Target, Qtype: dns.TypeA, Qclass: dns.ClassINET | 1<<15})
			}
		}
	}
	return out
}
