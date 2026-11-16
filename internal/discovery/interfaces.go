package discovery

import (
	"net"
	"path"
	"sort"
	"strings"
)

type interfaceRecord struct {
	iface net.Interface
	addrs []net.Addr
}

// Preference applies to the initial selection only. VPNs remain explicitly
// selectable for unusual setups. Routable physical adapters precede VPNs;
// link-local is only a last resort when no non-link-local address exists.
func availableInterfaces(records []interfaceRecord, kinds map[string]string) []Interface {
	out := []Interface{}
	for _, r := range records {
		ip, _, err := interfaceNetwork(r.iface, r.addrs)
		if err == nil {
			out = append(out, Interface{Name: r.iface.Name, Address: ip.String(), Kind: interfaceKind(r.iface.Name, ip, kinds[r.iface.Name])})
		}
	}
	rank := func(i Interface) int {
		rank := addressRank(net.ParseIP(i.Address))
		if rank == 2 {
			return 3
		}
		if i.Kind == "vpn" {
			return 2
		}
		return rank
	}
	// Within one address class a confirmed physical adapter precedes unknown
	// ones, e.g. virtual bridges such as bridge100 (VMs, Internet Sharing).
	known := func(i Interface) bool { return i.Kind == "wifi" || i.Kind == "ethernet" }
	sort.SliceStable(out, func(i, j int) bool {
		a, b := rank(out[i]), rank(out[j])
		if a != b {
			return a < b
		}
		if ka, kb := known(out[i]), known(out[j]); ka != kb {
			return ka
		}
		return out[i].Name < out[j].Name
	})
	return out
}
func addressRank(ip net.IP) int {
	if ip.IsPrivate() {
		return 0
	}
	if ip.IsLinkLocalUnicast() {
		return 2
	}
	return 1
}
func interfaceKind(name string, ip net.IP, hint string) string {
	lower := strings.ToLower(name)
	v4 := ip.To4()
	if strings.HasPrefix(lower, "utun") || strings.HasPrefix(lower, "tun") || strings.HasPrefix(lower, "tap") || strings.HasPrefix(lower, "tailscale") || strings.HasPrefix(lower, "wg") || v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return "vpn"
	}
	switch hint {
	case "wifi", "ethernet", "vpn":
		return hint
	}
	return "unknown"
}

// networksetup's stable Hardware Port / Device records, independent of order.
// Unknown ports stay unknown; an enN name alone says nothing about the hardware.
func parseHardwarePorts(output string) map[string]string {
	out := map[string]string{}
	kind := "unknown"
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if port, ok := strings.CutPrefix(line, "Hardware Port:"); ok {
			port = strings.ToLower(strings.TrimSpace(port))
			kind = "unknown"
			switch {
			case strings.Contains(port, "wi-fi"), strings.Contains(port, "airport"), strings.Contains(port, "wlan"):
				kind = "wifi"
			case strings.Contains(port, "ethernet"), strings.HasSuffix(port, " lan"):
				kind = "ethernet"
			}
		}
		if name, ok := strings.CutPrefix(line, "Device:"); ok {
			out[strings.TrimSpace(name)] = kind
		}
	}
	return out
}

// IANA ifType values used by Windows IP Helper and Linux ARPHRD_ETHER fallback.
func adapterKind(t uint32) string {
	switch t {
	case 6:
		return "ethernet"
	case 71:
		return "wifi"
	case 23, 131:
		return "vpn"
	}
	return "unknown"
}

func linuxInterfaceKinds(all []net.Interface, read func(string) ([]byte, error), exists func(string) bool) map[string]string {
	out := map[string]string{}
	for _, i := range all {
		base := path.Join("/sys/class/net", i.Name)
		switch {
		case exists(path.Join(base, "wireless")):
			out[i.Name] = "wifi"
		case exists(path.Join(base, "device")):
			// Only adapters backed by a device count as Ethernet; docker0,
			// virbr0, br-* and veth* are ARPHRD_ETHER too but purely virtual.
			data, err := read(path.Join(base, "type"))
			if err == nil && strings.TrimSpace(string(data)) == "1" {
				out[i.Name] = "ethernet"
			}
		}
	}
	return out
}
