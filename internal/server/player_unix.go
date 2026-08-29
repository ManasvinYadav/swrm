//go:build !windows

package server

import "syscall"

// detachedSysProcAttr starts the media player in its own session so it
// survives swrm exiting and doesn't hold swrm's controlling terminal.
func detachedSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
