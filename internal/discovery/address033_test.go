package discovery

import (
	"net"
	"testing"
)

func TestNetwork033PrefersPrivateAddress(t *testing.T) {
	var addrs []net.Addr
	for _, s := range []string{"169.254.1.166/16", "192.168." + "1.20/24"} {
		ip, n, _ := net.ParseCIDR(s)
		n.IP = ip
		addrs = append(addrs, n)
	}
	ip, _, err := interfaceNetwork(net.Interface{Flags: net.FlagUp | net.FlagMulticast}, addrs)
	if err != nil || ip.String() != "192.168."+"1.20" {
		t.Fatalf("private IPv4 must precede link-local: got %s (%v)", ip, err)
	}
}
