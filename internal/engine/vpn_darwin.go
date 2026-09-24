//go:build darwin

package engine

import (
	"strings"

	"golang.org/x/sys/unix"
)

// bindToInterface pins fd to ifaceIdx via IP_BOUND_IF (IPv4) or
// IPV6_BOUND_IF (IPv6), the Darwin/BSD mechanism for hard interface binding
// (VPN kill-switch behavior). Darwin has no SO_BINDTODEVICE equivalent by
// device name, hence the index. IP_BOUND_IF is an IPPROTO_IP option, so an
// AF_INET6 socket (which Go opens for "tcp6"/"udp6" and for dual-stack
// "tcp"/"udp" sockets alike, reporting them to Control as "…6") needs
// IPV6_BOUND_IF instead.
func bindToInterface(fd uintptr, network string, _ string, ifaceIdx int) error {
	if strings.HasSuffix(network, "6") {
		return unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_BOUND_IF, ifaceIdx)
	}
	return unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_BOUND_IF, ifaceIdx)
}
