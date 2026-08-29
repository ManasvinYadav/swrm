//go:build darwin

package engine

import "golang.org/x/sys/unix"

// bindToInterface pins fd to ifaceIdx via IP_BOUND_IF, the Darwin/BSD
// mechanism for hard interface binding (VPN kill-switch behavior). Darwin
// has no SO_BINDTODEVICE equivalent by device name, hence the index.
func bindToInterface(fd uintptr, _ string, ifaceIdx int) error {
	return unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_BOUND_IF, ifaceIdx)
}
