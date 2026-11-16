package eebus

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// DialRoute is a local discovery provenance, never supplied by the remote peer.
type DialRoute struct{ Interface, LocalAddress string }
type routeDialFunc func(context.Context, *net.Dialer, string, string) (net.Conn, error)
type bindingError struct{ error }

func (e *bindingError) Unwrap() error { return e.error }

func dialRoute(ctx context.Context, address string, route DialRoute, trace *connectionTrace, dial routeDialFunc) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ipText, zone, _ := strings.Cut(host, "%")
	ip := net.ParseIP(ipText)
	if ip == nil {
		return nil, errors.New("lokale IP-Adresse erforderlich")
	}
	required := ip.IsLinkLocalUnicast()
	if route.Interface == "" {
		route.Interface = zone
	}
	if zone != "" && route.Interface != zone && !sameRouteInterface(zone, route.Interface) {
		return nil, errors.New("IPv6-Zone passt nicht zum Adapter")
	}
	network := "tcp4"
	if ip.To4() == nil {
		network = "tcp6"
		if required {
			host = ipText + "%" + route.Interface
		}
	}
	d := &net.Dialer{}
	bound := route.LocalAddress != "" || route.Interface != ""
	if required && (route.LocalAddress == "" || route.Interface == "") {
		return nil, errors.New("Link-Local benötigt lokale Adresse und Adapter")
	}
	if bound {
		source := net.ParseIP(route.LocalAddress)
		if source == nil || (source.To4() != nil) != (ip.To4() != nil) {
			return nil, errors.New("lokale Adresse passt nicht zur Geräte-Adresse")
		}
		local := &net.TCPAddr{IP: source}
		if source.To4() == nil && source.IsLinkLocalUnicast() {
			local.Zone = route.Interface
		}
		d.LocalAddr = local
		d.Control = func(network, address string, c syscall.RawConn) error {
			if route.Interface == "" {
				return nil
			}
			iface, e := routeInterface(route.Interface)
			if e != nil {
				return &bindingError{e}
			}
			addrs, e := iface.Addrs()
			if e != nil {
				return &bindingError{e}
			}
			owned := false
			for _, a := range addrs {
				ip, _, e := net.ParseCIDR(a.String())
				if e == nil && ip.Equal(source) {
					owned = true
					break
				}
			}
			if !owned {
				return &bindingError{errors.New("lokale Adresse ist dem Adapter nicht mehr zugewiesen")}
			}
			var result error
			e = c.Control(func(fd uintptr) { result = bindInterface(fd, network, iface) })
			if e != nil {
				return &bindingError{e}
			}
			if result != nil {
				return &bindingError{result}
			}
			return nil
		}
	}
	if bound {
		trace.note(fmt.Sprintf("Lokale Adresse: %s; Adapter: %s", route.LocalAddress, route.Interface))
	} else {
		trace.note("Lokale Adresse und Adapter: System-Routing")
	}
	c, err := dial(ctx, d, network, net.JoinHostPort(host, port))
	var bindErr *bindingError
	var systemErr *os.SyscallError
	failedBinding := errors.As(err, &bindErr) || errors.As(err, &systemErr) && systemErr.Syscall == "bind"
	fellBack := false
	if err != nil && bound && !required && failedBinding && ctx.Err() == nil {
		fellBack = true
		trace.note(fmt.Sprintf("Adapterbindung fehlgeschlagen (%v); Rückfall auf System-Routing", err))
		c, err = dial(ctx, &net.Dialer{}, network, address)
	}
	if c != nil {
		actual := ""
		if addr, ok := c.LocalAddr().(*net.TCPAddr); ok {
			// 043: the bound adapter is reported only if binding succeeded (no fallback to system routing).
			boundOK := bound && route.Interface != "" && !fellBack
			actual = usedInterface(addr.IP, route.Interface, boundOK, systemInterfaceIPs())
		}
		trace.note("Verwendete lokale Adresse: " + c.LocalAddr().String() + "; Adapter: " + actual)
	}
	return c, err
}

type interfaceIPs struct {
	Name string
	IPs  []net.IP
}

func systemInterfaceIPs() []interfaceIPs {
	ifaces, _ := net.Interfaces()
	out := make([]interfaceIPs, 0, len(ifaces))
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		item := interfaceIPs{Name: iface.Name}
		for _, a := range addrs {
			if ip, _, e := net.ParseCIDR(a.String()); e == nil {
				item.IPs = append(item.IPs, ip)
			}
		}
		out = append(out, item)
	}
	return out
}

// usedInterface names the adapter of a local address. 043: several adapters may carry the same address
// (Ethernet and WLAN in one subnet); a successful binding decides, otherwise all owners are named.
func usedInterface(ip net.IP, bound string, boundOK bool, list []interfaceIPs) string {
	var owners []string
	for _, item := range list {
		for _, v := range item.IPs {
			if v.Equal(ip) {
				owners = append(owners, item.Name)
				break
			}
		}
	}
	if boundOK {
		for _, name := range owners {
			if name == bound || sameRouteInterface(name, bound) {
				return name
			}
		}
	}
	switch len(owners) {
	case 0:
		return ""
	case 1:
		return owners[0]
	}
	return strings.Join(owners, " oder ") + " (gleiche Adresse)"
}

// Windows commonly displays numeric IPv6 scope IDs; accept names and indices.
func routeInterface(name string) (*net.Interface, error) {
	if index, err := strconv.Atoi(name); err == nil {
		return net.InterfaceByIndex(index)
	}
	return net.InterfaceByName(name)
}
func sameRouteInterface(a, b string) bool {
	left, e := routeInterface(a)
	if e != nil {
		return false
	}
	right, e := routeInterface(b)
	return e == nil && left.Index == right.Index
}
