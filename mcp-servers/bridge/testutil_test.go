package main

import (
	"encoding/json"
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
	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil { //nolint:gosec
		t.Fatalf("makeSlowScript: %v", err)
	}
	return scriptPath
}

// makeNamedExitScript creates a shell script with the given name that exits
// immediately with exitCode. Returns the full path to the script.
func makeNamedExitScript(t *testing.T, name string, exitCode int) string {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, name)
	content := "#!/bin/sh\nexit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil { //nolint:gosec
		t.Fatalf("makeNamedExitScript: %v", err)
	}
	return scriptPath
}

// makeFastExitScript creates a shell script that exits immediately with exitCode.
// Useful for testing non-hang failure modes (immediate CLI error return).
func makeFastExitScript(t *testing.T, exitCode int) string {
	return makeNamedExitScript(t, "fake-cli", exitCode)
}

// writeTestConfig writes c as JSON to the config path under home so that
// reloadState() loads it correctly during tests that set HOME to a temp dir.
func writeTestConfig(t *testing.T, home string, c Config) {
	t.Helper()
	configDir := filepath.Join(home, ".config", "oh-my-bridge")
	if err := os.MkdirAll(configDir, 0750); err != nil {
		t.Fatalf("writeTestConfig MkdirAll: %v", err)
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("writeTestConfig Marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), data, 0600); err != nil {
		t.Fatalf("writeTestConfig WriteFile: %v", err)
	}
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
		lines += fmt.Sprintf("sleep %.3f\n", float64(intervalMs)/1000.0)
	}
	lines += "sleep " + strconv.Itoa(finalSleepSec) + "\n"
	if err := os.WriteFile(scriptPath, []byte(lines), 0755); err != nil { //nolint:gosec
		t.Fatalf("makeIncrementalOutputScript: %v", err)
	}
	return scriptPath
}
