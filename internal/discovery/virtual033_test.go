package discovery

import (
	"net"
	"testing"
)

// Virtual bridges (Colima/UTM/Internet Sharing bridge100, docker0, virbr0) carry
// RFC1918 addresses too. A known physical adapter must still win the preselection.
func TestInterfaces033VirtualBridgeDoesNotWin(t *testing.T) {
	records := []interfaceRecord{
		{net.Interface{Name: "bridge100", Flags: net.FlagUp | net.FlagMulticast}, []net.Addr{cidr033("192.168." + "64.1/24")}},
		{net.Interface{Name: "en1", Flags: net.FlagUp | net.FlagMulticast}, []net.Addr{cidr033("192.168." + "1.20/24")}},
	}
	got := availableInterfaces(records, parseHardwarePorts("Hardware Port: Wi-Fi\nDevice: en1\n"))
	if got[0].Name != "en1" {
		t.Fatalf("physical Wi-Fi must precede an unknown bridge: %+v", got)
	}
}

func TestInterfaces033LinuxVirtualEthernet(t *testing.T) {
	all := []net.Interface{{Name: "docker0"}, {Name: "eth0"}}
	got := linuxInterfaceKinds(all, func(string) ([]byte, error) { return []byte("1\n"), nil },
		func(path string) bool { return path == "/sys/class/net/eth0/device" })
	if got["docker0"] != "" || got["eth0"] != "ethernet" {
		t.Fatal(got)
	}
}
