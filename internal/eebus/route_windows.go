package eebus

import (
	"math/bits"
	"net"
	"syscall"
)

// Winsock IP_UNICAST_IF expects the IPv4 index in network byte order;
// IPV6_UNICAST_IF expects host byte order. Both supported Windows targets are LE.
func bindInterface(fd uintptr, network string, iface *net.Interface) error {
	level, index := syscall.IPPROTO_IP, int(bits.ReverseBytes32(uint32(iface.Index)))
	if network == "tcp6" {
		level, index = syscall.IPPROTO_IPV6, iface.Index
	}
	return syscall.SetsockoptInt(syscall.Handle(fd), level, 31, index)
}
