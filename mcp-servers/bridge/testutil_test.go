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
