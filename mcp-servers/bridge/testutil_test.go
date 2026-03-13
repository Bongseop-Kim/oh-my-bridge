package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// makeSlowScript creates a shell script that sleeps for the given number of seconds.
// Useful for testing timeout behaviour and first-output-timeout (no output produced).
func makeSlowScript(t *testing.T, seconds int) string {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "slow-cli")
	content := "#!/bin/sh\nsleep " + strconv.Itoa(seconds) + "\n"
	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
		t.Fatalf("makeSlowScript: %v", err)
	}
	return scriptPath
}

// makeFastExitScript creates a shell script that exits immediately with exitCode.
// Useful for testing non-hang failure modes (immediate CLI error return).
func makeFastExitScript(t *testing.T, exitCode int) string {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "fake-cli")
	content := "#!/bin/sh\nexit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
		t.Fatalf("makeFastExitScript: %v", err)
	}
	return scriptPath
}

// makeIncrementalOutputScript creates a script that emits `chunks` lines at
// `intervalMs` ms intervals, then sleeps for `finalSleepSec` seconds.
// Useful for testing stability-timeout behaviour.
func makeIncrementalOutputScript(t *testing.T, chunks int, intervalMs int, finalSleepSec int) string {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "incremental-cli")
	lines := "#!/bin/sh\n"
	for i := 0; i < chunks; i++ {
		lines += "echo chunk" + strconv.Itoa(i) + "\n"
		lines += fmt.Sprintf("sleep 0.%03d\n", intervalMs)
	}
	lines += "sleep " + strconv.Itoa(finalSleepSec) + "\n"
	if err := os.WriteFile(scriptPath, []byte(lines), 0755); err != nil {
		t.Fatalf("makeIncrementalOutputScript: %v", err)
	}
	return scriptPath
}
