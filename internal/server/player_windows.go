//go:build windows

package server

import "syscall"

// DETACHED_PROCESS (Win32 CreateProcess creation flag) isn't exposed as a
// constant by the standard syscall package on windows/amd64.
const detachedProcess = 0x00000008

// detachedSysProcAttr starts the media player detached from swrm's console
// so it survives swrm exiting and doesn't hold swrm's controlling terminal.
func detachedSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | detachedProcess}
}
