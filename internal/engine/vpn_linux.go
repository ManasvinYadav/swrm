//go:build linux

package engine

import "golang.org/x/sys/unix"

// bindToInterface pins fd to ifaceName via SO_BINDTODEVICE, the Linux
// mechanism for hard interface binding (VPN kill-switch behavior).
func bindToInterface(fd uintptr, ifaceName string, _ int) error {
	return unix.SetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE, ifaceName)
}
