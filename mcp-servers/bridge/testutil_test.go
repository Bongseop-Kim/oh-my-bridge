package main

import (
	"os"
	"path/filepath"
	"testing"
)

// makeSlowScript creates a shell script that sleeps for the given number of seconds.
// Useful for testing timeout behaviour.
func makeSlowScript(t *testing.T, seconds int) string {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "slow-cli")
	content := "#!/bin/sh\nsleep " + itoa(seconds) + "\n"
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
	content := "#!/bin/sh\nexit " + itoa(exitCode) + "\n"
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
	// Build the script body
	lines := "#!/bin/sh\n"
	for i := 0; i < chunks; i++ {
		lines += "echo chunk" + itoa(i) + "\n"
		lines += "sleep 0." + zeroPad(intervalMs, 3) + "\n"
	}
	lines += "sleep " + itoa(finalSleepSec) + "\n"
	if err := os.WriteFile(scriptPath, []byte(lines), 0755); err != nil {
		t.Fatalf("makeIncrementalOutputScript: %v", err)
	}
	return scriptPath
}

// makeNoOutputScript creates a script that sleeps without producing any output.
// Useful for testing first-output-timeout behaviour.
func makeNoOutputScript(t *testing.T, sleepSec int) string {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "no-output-cli")
	content := "#!/bin/sh\nsleep " + itoa(sleepSec) + "\n"
	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
		t.Fatalf("makeNoOutputScript: %v", err)
	}
	return scriptPath
}

// zeroPad returns n as a zero-padded decimal string of exactly `width` digits.
func zeroPad(n, width int) string {
	s := itoa(n)
	for len(s) < width {
		s = "0" + s
	}
	return s
}

// itoa converts a non-negative integer to its decimal string representation.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}
