package discovery

import (
	"net"
	"os"
)

func platformInterfaceKinds(all []net.Interface) map[string]string {
	return linuxInterfaceKinds(all, os.ReadFile, func(path string) bool { _, err := os.Stat(path); return err == nil })
}
