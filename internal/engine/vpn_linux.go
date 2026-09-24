//go:build linux

package engine

import "golang.org/x/sys/unix"

// bindToInterface pins fd to ifaceName via SO_BINDTODEVICE, the Linux
// mechanism for hard interface binding (VPN kill-switch behavior). It
// applies to IPv4 and IPv6 sockets alike.
func bindToInterface(fd uintptr, _ string, ifaceName string, _ int) error {
	return unix.SetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE, ifaceName)
}
