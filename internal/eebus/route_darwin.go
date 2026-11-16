package eebus

import (
	"net"
	"syscall"
)

func bindInterface(fd uintptr, network string, iface *net.Interface) error {
	level, option := syscall.IPPROTO_IP, syscall.IP_BOUND_IF
	if network == "tcp6" {
		level, option = syscall.IPPROTO_IPV6, syscall.IPV6_BOUND_IF
	}
	return syscall.SetsockoptInt(int(fd), level, option, iface.Index)
}
