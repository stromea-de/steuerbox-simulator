package discovery

import (
	"errors"
	"net"
	"testing"
)

func TestInterfaces033LinuxHardware(t *testing.T) {
	all := []net.Interface{{Name: "wifi"}, {Name: "eth0"}, {Name: "missing"}}
	got := linuxInterfaceKinds(all, func(path string) ([]byte, error) {
		if path == "/sys/class/net/eth0/type" {
			return []byte("1\n"), nil
		}
		return nil, errors.New("missing")
	}, func(path string) bool {
		return path == "/sys/class/net/wifi/wireless" || path == "/sys/class/net/eth0/device"
	})
	if got["wifi"] != "wifi" || got["eth0"] != "ethernet" || got["missing"] != "" {
		t.Fatal(got)
	}
}
