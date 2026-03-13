//go:build unix

package main

import (
	"os/exec"
	"syscall"
)

// setupProc puts the process in its own process group so SIGTERM reaches all
// descendant processes (e.g. grandchild `sleep` spawned by a shell script).
// Without this, grandchildren keep the pipe write-end open and io.Copy
// never returns EOF.
func setupProc(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// Negative PID sends to the entire process group.
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
}
