//go:build windows

package main

import "os/exec"

// setupProc sets a safe Cancel function on Windows.
// Process groups and SIGTERM are not available; kill the process directly.
func setupProc(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return cmd.Process.Kill()
	}
}
