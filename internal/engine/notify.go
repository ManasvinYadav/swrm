package engine

import (
	"os/exec"
)

func RunPostDownload(cmdStr string) {
	if cmdStr == "" {
		return
	}
	_ = exec.Command("sh", "-c", cmdStr).Start()
}
