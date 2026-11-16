// Package discovery implements isolated SHIP DNS-SD and explicit read-only LAN discovery.
package discovery

import (
	"context"
	"errors"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
	"github.com/stromea-de/steuerbox-simulator/internal/eebus"
	"github.com/stromea-de/steuerbox-simulator/internal/identity"
	"golang.org/x/net/ipv4"
)

const Service = "_ship._tcp.local."

type Device struct {
	Name         string `json:"name"`
	Host         string `json:"host,omitempty"`
	ServiceName  string `json:"serviceName,omitempty"`
	SKI          string `json:"ski"`
	Brand        string `json:"brand"`
	Model        string `json:"model"`
	Address      string `json:"address"`
	Interface    string `json:"interface,omitempty"`
	LocalAddress string `json:"localAddress,omitempty"`
}
type Responder struct {
	conn   *net.UDPConn
	hidden atomic.Bool
}

// SetVisible controls announcements of this isolated loopback fixture only.
func (r *Responder) SetVisible(visible bool) { r.hidden.Store(!visible) }

func (r *Responder) Address() string { return r.conn.LocalAddr().String() }
func (r *Responder) Close() error    { return r.conn.Close() }

func StartResponder(d Device) (*Responder, error) {
	if err := eebus.LoopbackAddress(d.Address); err != nil {
		return nil, err
	}
	if _, err := identity.NormalizeSKI(d.SKI); err != nil {
		return nil, err
	}
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		return nil, err
	}
	r := &Responder{conn: c}
	go func() {
		b := make([]byte, 9000)
		for {
			n, peer, err := c.ReadFromUDP(b)
			if err != nil {
				return
			}
			if !peer.IP.IsLoopback() {
				continue
			}
			var q dns.Msg
			if q.Unpack(b[:n]) != nil {
				continue
			}
			valid := false
			for _, question := range q.Question {
				if question.Name == Service && question.Qtype == dns.TypePTR {
					valid = true
				}
			}
			if !valid {
				continue
			}
			if r.hidden.Load() {
				continue // An unannounced fixture sends no service records.
			}
			reply := records(d)
			reply.Id = q.Id
			reply.Response = true
			reply.Authoritative = true
			data, err := reply.Pack()
			if err == nil {
				_, _ = c.WriteToUDP(data, peer)
			}
		}
	}()
	return r, nil
}
func records(d Device) *dns.Msg {
	host, port, _ := net.SplitHostPort(d.Address)
	p, _ := strconv.Atoi(port)
	instance := "Stromea-Steuerbox-Simulator-" + d.SKI[:8] + "." + Service
	target := "sim-" + d.SKI[:8] + ".local."
	h := func(name string, t uint16) dns.RR_Header {
		return dns.RR_Header{Name: name, Rrtype: t, Class: dns.ClassINET, Ttl: 120}
	}
	return &dns.Msg{Answer: []dns.RR{&dns.PTR{Hdr: h(Service, dns.TypePTR), Ptr: instance}}, Extra: []dns.RR{
		&dns.SRV{Hdr: h(instance, dns.TypeSRV), Port: uint16(p), Target: target},
		&dns.TXT{Hdr: h(instance, dns.TypeTXT), Txt: []string{"txtvers=1", "id=" + d.SKI, "ski=" + d.SKI, "brand=" + d.Brand, "model=" + d.Model, "name=" + d.Name, "path=/ship/", "register=false"}},
		&dns.A{Hdr: h(target, dns.TypeA), A: net.ParseIP(host)},
	}}
}

// Search sends DNS-SD packets directly to an isolated simulator responder.
func Search(ctx context.Context, addr string) ([]Device, error) {
	if err := eebus.LoopbackAddress(addr); err != nil {
		return nil, err
	}
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		return nil, err
	}
	defer c.Close()
	peer, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return nil, err
	}
	return exchange(ctx, c, peer, false)
}

// SearchMDNS sends a QU mDNS question on the loopback interface only.
// The unicast-response bit lets responses return to this loopback-bound ephemeral socket.
func SearchMDNS(ctx context.Context) ([]Device, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var loop *net.Interface
	for i := range ifaces {
		if ifaces[i].Flags&net.FlagLoopback != 0 && ifaces[i].Flags&net.FlagUp != 0 {
			loop = &ifaces[i]
			break
		}
	}
	if loop == nil {
		return nil, errors.New("kein Loopback-Interface")
	}
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		return nil, err
	}
	defer c.Close()
	p := ipv4.NewPacketConn(c)
	if err = p.SetMulticastInterface(loop); err != nil {
		return nil, err
	}
	if err = p.SetMulticastTTL(255); err != nil {
		return nil, err
	}
	if err = p.SetMulticastLoopback(true); err != nil {
		return nil, err
	}
	return exchange(ctx, c, &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}, true)
}
func exchange(ctx context.Context, c *net.UDPConn, peer *net.UDPAddr, multicast bool) ([]Device, error) {
	return exchangeAllowed(ctx, c, peer, multicast, func(ip net.IP) bool { return ip.IsLoopback() })
}

func exchangeAllowed(ctx context.Context, c *net.UDPConn, peer *net.UDPAddr, multicast bool, allowed func(net.IP) bool, observers ...func([]Device)) ([]Device, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	q := new(dns.Msg)
	q.SetQuestion(Service, dns.TypePTR)
	q.RecursionDesired = false
	if multicast {
		q.Id = 0
		q.Question[0].Qclass |= 1 << 15
	}
	b, err := q.Pack()
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(6 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err = c.SetDeadline(deadline); err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = c.Close() })
	defer stop()
	if _, err = c.WriteToUDP(b, peer); err != nil {
		return nil, err
	}
	out := []Device{}
	cache := newRecordCache()
	requested := map[string]bool{}
	buf := make([]byte, 9000)
	retryInterval := time.Second
	nextQuery := time.Now().Add(retryInterval)
	for {
		readUntil := nextQuery
		if deadline.Before(readUntil) {
			readUntil = deadline
		}
		_ = c.SetReadDeadline(readUntil)
		n, from, err := c.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return parseAllowed(cache.message(), allowed), ctx.Err()
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				if !time.Now().Before(deadline) {
					return parseAllowed(cache.message(), allowed), nil
				}
				retry := q.Copy()
				if multicast {
					retry.Answer = cache.message().Answer
				}
				packet, packErr := retry.Pack()
				if packErr != nil {
					return nil, packErr
				}
				if _, err = c.WriteToUDP(packet, peer); err != nil {
					return nil, err
				}
				requested = map[string]bool{}
				retryInterval *= 2
				nextQuery = time.Now().Add(retryInterval)
				continue
			}
			return nil, err
		}
		expectedPort := peer.Port
		if multicast && peer.IP.IsMulticast() {
			expectedPort = 5353
		}
		if !allowed(from.IP) || from.Port != expectedPort || (!multicast && !from.IP.Equal(peer.IP)) {
			continue
		}
		var reply dns.Msg
		if reply.Unpack(buf[:n]) != nil || !reply.Response || reply.Rcode != dns.RcodeSuccess || reply.Truncated || (!multicast && reply.Id != q.Id) {
			continue
		}
		cache.add(&reply)
		out = parseAllowed(cache.message(), allowed)
		for _, update := range observers {
			if update != nil {
				update(out)
			}
		}
		if !multicast {
			return out, nil
		}
		// DNS-SD responders may split PTR, SRV/TXT and address records.
		for _, question := range cache.questions() {
			key := strings.ToLower(question.Name) + "/" + strconv.Itoa(int(question.Qtype))
			if requested[key] {
				continue
			}
			requested[key] = true
			follow := &dns.Msg{Question: []dns.Question{question}}
			packet, packErr := follow.Pack()
			if packErr != nil {
				return nil, packErr
			}
			if _, err = c.WriteToUDP(packet, peer); err != nil {
				return nil, err
			}
		}
	}
}
func parse(m *dns.Msg) []Device {
	return parseAllowed(m, func(ip net.IP) bool { return ip.IsLoopback() })
}
func parseAllowed(m *dns.Msg, allowed func(net.IP) bool) []Device {
	rr := append(append([]dns.RR{}, m.Answer...), m.Extra...)
	out := []Device{}
	for _, r := range rr {
		ptr, ok := r.(*dns.PTR)
		if !ok || !strings.EqualFold(ptr.Hdr.Name, Service) || ptr.Hdr.Ttl == 0 {
			continue
		}
		d := Device{Name: strings.TrimSuffix(ptr.Ptr, "."+Service), ServiceName: ptr.Ptr}
		var target string
		var port uint16
		for _, r := range rr {
			switch v := r.(type) {
			case *dns.TXT:
				if strings.EqualFold(v.Hdr.Name, ptr.Ptr) {
					for _, s := range v.Txt {
						key, value, ok := strings.Cut(s, "=")
						if !ok {
							continue
						}
						switch key {
						case "ski":
							d.SKI = value
						case "brand":
							d.Brand = value
						case "model":
							d.Model = value
						case "name":
							d.Name = value
						}
					}
				}
			case *dns.SRV:
				if strings.EqualFold(v.Hdr.Name, ptr.Ptr) {
					target = v.Target
					port = v.Port
				}
			}
		}
		d.Host = target
		ski, err := identity.NormalizeSKI(d.SKI)
		if err != nil {
			continue
		}
		d.SKI = ski
		for _, r := range rr {
			v, ok := r.(*dns.A)
			if ok && strings.EqualFold(v.Hdr.Name, target) && allowed(v.A) && !v.A.IsUnspecified() && !v.A.IsMulticast() {
				d.Address = net.JoinHostPort(v.A.String(), strconv.Itoa(int(port)))
				break
			}
		}
		if d.Address != "" && port != 0 {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SKI+out[i].Address < out[j].SKI+out[j].Address })
	return out
}
