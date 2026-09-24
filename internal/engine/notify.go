package engine

import (
	"os/exec"
	"runtime"
)

// RunPostDownload starts cmdStr through the platform shell without waiting
// for it. Its stdio is left unset (the null device), so it can't draw over
// the TUI.
func RunPostDownload(cmdStr string) {
	if cmdStr == "" {
		return
	}
	cmd := exec.Command("sh", "-c", cmdStr)
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", cmdStr)
	}
	if err := cmd.Start(); err != nil {
		return
	}
	// Reap it so a finished command doesn't linger as a zombie process for
	// the rest of the session.
	go cmd.Wait()
}
