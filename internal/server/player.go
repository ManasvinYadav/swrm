package server

import (
	"fmt"
	"os/exec"
	"syscall"
)

func LaunchPlayer(streamURL string, preferred string) (string, error) {
	players := []string{"mpv", "vlc", "iina"}
	if preferred != "" && preferred != "auto" {
		players = append([]string{preferred}, players...)
	}

	for _, p := range players {
		path, err := exec.LookPath(p)
		if err == nil {
			var cmd *exec.Cmd
			if p == "mpv" {
				cmd = exec.Command(path, "--demuxer-lavf-o=probesize=32768,analyzeduration=0", "--force-seekable=yes", streamURL)
			} else {
				cmd = exec.Command(path, streamURL)
			}

			// Detach into its own session so the player survives swrm exiting
			// and doesn't hold swrm's controlling terminal, and discard its
			// stdio so it can't block on a closed pipe.
			cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			cmd.Stdin = nil
			cmd.Stdout = nil
			cmd.Stderr = nil

			if err := cmd.Start(); err != nil {
				continue
			}
			_ = cmd.Process.Release()
			return p, nil
		}
	}

	return "", fmt.Errorf("no suitable media player found")
}
