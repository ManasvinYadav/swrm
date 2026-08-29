package engine

import (
	"os/exec"
	"runtime"
)

func Notify(title, message string) {
	if runtime.GOOS == "darwin" {
		_ = exec.Command("osascript", "-e", "on run argv\ndisplay notification (item 1 of argv) with title (item 2 of argv)\nend run", message, title).Start()
	} else if runtime.GOOS == "linux" {
		_ = exec.Command("notify-send", title, message).Start()
	}
}

func RunPostDownload(cmdStr string) {
	if cmdStr == "" {
		return
	}
	_ = exec.Command("sh", "-c", cmdStr).Start()
}
