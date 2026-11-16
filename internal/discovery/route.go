package discovery

import (
	"errors"
	"net"
	"strconv"
	"strings"
)

type localNetwork struct{ name, cidr string }

// ResolveDeviceRoute enumerates local addresses only; it sends no packets.
// A discovered source remains pinned even if that adapter later disappears.
func ResolveDeviceRoute(d Device) (Device, error) {
	if d.LocalAddress != "" {
		return d, nil
	}
	host, _, err := net.SplitHostPort(d.Address)
	if err != nil {
		return d, err
	}
	text, zone, _ := strings.Cut(host, "%")
	if net.ParseIP(text).IsLoopback() && d.Interface == "" {
		return d, nil
	}
	all, err := net.Interfaces()
	if err != nil {
		return d, err
	}
	var candidates []localNetwork
	for _, iface := range all {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, e := iface.Addrs()
		if e != nil {
			continue
		}
		name := iface.Name
		if zone == strconv.Itoa(iface.Index) {
			if d.Interface == iface.Name {
				d.Interface = zone
			}
			name = zone
		}
		for _, a := range addrs {
			candidates = append(candidates, localNetwork{name, a.String()})
		}
	}
	return resolveDeviceRoute(d, candidates)
}
func resolveDeviceRoute(d Device, candidates []localNetwork) (Device, error) {
	host, _, err := net.SplitHostPort(d.Address)
	if err != nil {
		return d, err
	}
	text, zone, _ := strings.Cut(host, "%")
	target := net.ParseIP(text)
	if target == nil {
		return d, errors.New("IP-Adresse erforderlich")
	}
	if zone != "" {
		if d.Interface != "" && d.Interface != zone {
			return d, errors.New("IPv6-Zone passt nicht zum Adapter")
		}
		d.Interface = zone
	}
	best := -1
	matches := map[string]bool{}
	chosen := d
	for _, candidate := range candidates {
		if d.Interface != "" && candidate.name != d.Interface {
			continue
		}
		ip, subnet, e := net.ParseCIDR(candidate.cidr)
		if e != nil || (ip.To4() != nil) != (target.To4() != nil) {
			continue
		}
		prefix, _ := subnet.Mask.Size()
		if !subnet.Contains(target) {
			if d.Interface == "" {
				continue
			}
			prefix = 0
		}
		if prefix > best {
			best = prefix
			matches = map[string]bool{}
			chosen = d
			chosen.Interface = candidate.name
			chosen.LocalAddress = ip.String()
		}
		if prefix == best {
			matches[candidate.name] = true
		}
	}
	if target.IsLinkLocalUnicast() && (len(matches) == 0 || len(matches) > 1 && d.Interface == "") {
		return d, errors.New("Link-Local: passenden Adapter ausdrücklich wählen")
	}
	if len(matches) == 0 && d.Interface != "" {
		return d, errors.New("Adapter hat keine passende lokale Adresse")
	}
	return chosen, nil
}
func withRoute(devices []Device, name, local string) []Device {
	for i := range devices {
		devices[i].Interface = name
		devices[i].LocalAddress = local
	}
	return devices
}
