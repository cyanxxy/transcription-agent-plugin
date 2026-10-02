//go:build !windows

package main

import (
	"os/exec"
	"syscall"
	"time"
)

// The wrapper launches an engine and FFmpeg. Cancelling only the wrapper
// would let those children continue running and hold the output pipes open.
func configureCancellation(cmd *exec.Cmd) func() {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var killTimer *time.Timer
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		killTimer = time.AfterFunc(2*time.Second, func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) })
		return err
	}
	return func() {
		// Cmd.Run waits for its context watcher before returning, so the timer
		// assignment and this cleanup do not race.
		if killTimer != nil {
			killTimer.Stop()
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}
}
