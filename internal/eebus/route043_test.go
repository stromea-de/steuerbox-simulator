package eebus

import (
	"net"
	"testing"
)

// 043 (Echtgerät 2026-11-14): Ethernet (en0) and WLAN (en1) carried the same IPv4 address. The connection was
// bound to en1, but the log named the first interface with that address ("Adapter: en0").
func TestUsedInterfacePrefersBoundAdapter043(t *testing.T) {
	ip := net.ParseIP("192.0.2.20")
	list := []interfaceIPs{
		{Name: "en0", IPs: []net.IP{net.ParseIP("192.0.2.20")}},
		{Name: "en1", IPs: []net.IP{net.ParseIP("192.0.2.20")}},
	}
	if got := usedInterface(ip, "en1", true, list); got != "en1" {
		t.Fatalf("bound adapter not reported: %q", got)
	}
	// Without a successful binding (system routing) the owner is ambiguous and must be said so.
	if got := usedInterface(ip, "", false, list); got != "en0 oder en1 (gleiche Adresse)" {
		t.Fatalf("ambiguous owner not named: %q", got)
	}
	// After a failed binding (fallback to system routing) the bound name must not be claimed.
	if got := usedInterface(ip, "en1", false, list); got != "en0 oder en1 (gleiche Adresse)" {
		t.Fatalf("fallback claimed the bound adapter: %q", got)
	}
	if got := usedInterface(ip, "", false, list[:1]); got != "en0" {
		t.Fatalf("single owner: %q", got)
	}
	if got := usedInterface(net.ParseIP("198.51.100.1"), "", false, list); got != "" {
		t.Fatalf("unknown address: %q", got)
	}
}
