package discovery

import "testing"

func TestRoute028ManualSubnet(t *testing.T) {
	candidates := []localNetwork{{"vpn", "169.254.83.107/16"}, {"Ethernet", "169.254.78.1/16"}, {"Ethernet", "192.0.2.1/24"}, {"Ethernet", "fe80::1/64"}}
	for _, tc := range []struct {
		address, iface, local string
		fail                  bool
	}{
		{"169.254.78.213:4711", "Ethernet", "169.254.78.1", false},
		{"169.254.78.213:4711", "", "", true},
		{"192.0.2.2:4711", "", "192.0.2.1", false},
		{"[fe80::2%Ethernet]:4711", "", "fe80::1", false},
		{"169.254.78.213:4711", "missing", "", true},
	} {
		d, e := resolveDeviceRoute(Device{Address: tc.address, Interface: tc.iface}, candidates)
		if (e != nil) != tc.fail || e == nil && d.LocalAddress != tc.local {
			t.Fatalf("%+v => %+v %v", tc, d, e)
		}
	}
}

func TestRoute028DiscoveryProvenance(t *testing.T) {
	got := withRoute([]Device{{Address: "169.254.78.213:4711"}}, "Ethernet", "169.254.78.1")
	if got[0].Interface != "Ethernet" || got[0].LocalAddress != "169.254.78.1" {
		t.Fatal(got)
	}
}
