package eebus

import (
	"net"
	"syscall"
)

func bindInterface(fd uintptr, _ string, iface *net.Interface) error {
	return syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, iface.Name)
}
