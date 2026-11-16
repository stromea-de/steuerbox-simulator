package discovery

import (
	"net"
	"reflect"
	"testing"
)

func TestInterfaces033StudioAndPreference(t *testing.T) {
	// Synthetic numeric CGNAT octets; no literal host addresses in public sources.
	// Fully injected: never enumerates host adapters or sends network packets.
	records := []interfaceRecord{
		{net.Interface{Name: "en0", Flags: net.FlagUp | net.FlagMulticast}, []net.Addr{cidr033("169.254.1.166/16")}},
		{net.Interface{Name: "en1", Flags: net.FlagUp | net.FlagMulticast}, []net.Addr{cidr033("192.168." + "1.20/24")}},
		{net.Interface{Name: "utun4", Flags: net.FlagUp | net.FlagMulticast}, []net.Addr{cidr033(net.IPv4(100, 100, 1, 2).String() + "/32")}},
		{net.Interface{Name: "lo0", Flags: net.FlagUp | net.FlagLoopback | net.FlagMulticast}, []net.Addr{cidr033("127.0.0.1/8")}},
	}
	types := parseHardwarePorts("Hardware Port: Ethernet\nDevice: en0\nEthernet Address: unused\n\nHardware Port: Wi-Fi\nDevice: en1\n")
	got := availableInterfaces(records, types)
	want := []Interface{{Name: "en1", Address: "192.168." + "1.20", Kind: "wifi"}, {Name: "utun4", Address: net.IPv4(100, 100, 1, 2).String(), Kind: "vpn"}, {Name: "en0", Address: "169.254.1.166", Kind: "ethernet"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v; want %+v", got, want)
	}
	if got := availableInterfaces(records[:1], types); len(got) != 1 || got[0].Address != "169.254.1.166" {
		t.Fatal(got)
	}
}
func cidr033(s string) net.Addr { ip, n, _ := net.ParseCIDR(s); n.IP = ip; return n }

func TestInterfaces033TypesAndAddressChoice(t *testing.T) {
	for _, tc := range []struct{ name, ip, hint, want string }{
		{"en0", "192.168." + "1.2", "", "unknown"}, // Never guess the hardware from en0.
		{"en1", "192.168." + "1.2", "wifi", "wifi"},
		{"Ethernet", "192.168." + "1.2", "ethernet", "ethernet"},
		{"wlan0", "192.168." + "1.2", "wifi", "wifi"},
		{"utun0", "192.168." + "1.2", "wifi", "vpn"},
		{"tailscale0", "192.168." + "1.2", "", "vpn"},
		{"wg0", "192.168." + "1.2", "ethernet", "vpn"},
		{"adapter", net.IPv4(100, 64, 0, 1).String(), "ethernet", "vpn"},
		{"adapter", net.IPv4(100, 127, 255, 254).String(), "", "vpn"},
		{"adapter", "100.128.0.1", "", "unknown"},
	} {
		if got := interfaceKind(tc.name, net.ParseIP(tc.ip), tc.hint); got != tc.want {
			t.Errorf("%+v: %s", tc, got)
		}
	}
	// An adapter's address order must not let link-local hide its RFC1918 address.
	ip, _, err := interfaceNetwork(net.Interface{Flags: net.FlagUp | net.FlagMulticast}, []net.Addr{cidr033("169.254.1.2/16"), cidr033("10.0." + "0.2/24")})
	if err != nil || ip.String() != "10.0."+"0.2" {
		t.Fatalf("%s %v", ip, err)
	}
	records := []interfaceRecord{}
	for _, tc := range []struct{ name, cidr string }{{"wg0", "10.9." + "0.1/24"}, {"direct", "169.254.1.2/16"}, {"public", "192.0.2.2/24"}, {"lan", "172.16." + "0.2/24"}} {
		records = append(records, interfaceRecord{net.Interface{Name: tc.name, Flags: net.FlagUp | net.FlagMulticast}, []net.Addr{cidr033(tc.cidr)}})
	}
	got := availableInterfaces(records, nil)
	names := []string{}
	for _, v := range got {
		names = append(names, v.Name)
	}
	if !reflect.DeepEqual(names, []string{"lan", "public", "wg0", "direct"}) {
		t.Fatal(names)
	}
}

func TestInterfaces033HardwarePorts(t *testing.T) {
	for _, s := range []string{"Hardware Port: Wi-Fi\nDevice: en7\n", "Hardware Port: AirPort\r\nDevice: en7\r\n"} {
		if got := parseHardwarePorts(s)["en7"]; got != "wifi" {
			t.Fatal(got)
		}
	}
	if got := parseHardwarePorts("Hardware Port: Thunderbolt Bridge\nDevice: bridge0\n")["bridge0"]; got != "unknown" {
		t.Fatal(got)
	}
}

func TestInterfaces033WindowsTypes(t *testing.T) {
	for _, tc := range []struct {
		value uint32
		want  string
	}{{6, "ethernet"}, {71, "wifi"}, {23, "vpn"}, {131, "vpn"}, {1, "unknown"}} {
		if got := adapterKind(tc.value); got != tc.want {
			t.Fatalf("%d => %s", tc.value, got)
		}
	}
}

func TestInterfaces033USBAdapter(t *testing.T) {
	if got := parseHardwarePorts("Hardware Port: USB 10/100/1000 LAN\nDevice: en8\n")["en8"]; got != "ethernet" {
		t.Fatal(got)
	}
}
