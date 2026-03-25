package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// escapeForSingleQuotedShell replaces each single-quote in s with '\”.
// Use this before embedding a value inside a single-quoted shell literal.
func escapeForSingleQuotedShell(s string) string {
	return strings.ReplaceAll(s, "'", `'\''`)
}

// prependDirToPath prepends dir to PATH for the duration of the test.
func prependDirToPath(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func makeNamedProxyScript(t *testing.T, name, target string) string {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, name)
	content := fmt.Sprintf("#!/bin/sh\nexec '%s' \"$@\"\n", escapeForSingleQuotedShell(target))
	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil { //nolint:gosec
		t.Fatalf("makeNamedProxyScript: %v", err)
	}
	return scriptPath
}

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

// saveAndRestoreState saves the current global cfg and availableCLIs and
// registers a cleanup to restore them. Call before writeTestConfig+reloadState
// in any test that mutates global state.
func saveAndRestoreState(t *testing.T) {
	t.Helper()
	mu.Lock()
	origCfg := cfg
	origCLIs := availableCLIs
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		cfg = origCfg
		availableCLIs = origCLIs
		mu.Unlock()
	})
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

// makeStderrOnlyScript creates a script that writes `chunks` lines to stderr at
// `intervalMs` ms intervals, then sleeps for `finalSleepSec` seconds.
// Stdout is never written — simulates a CLI that uses stderr for progress.
func makeStderrOnlyScript(t *testing.T, chunks, intervalMs, finalSleepSec int) string {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "stderr-only-cli")
	lines := "#!/bin/sh\n"
	for i := 0; i < chunks; i++ {
		lines += fmt.Sprintf("echo chunk%d >&2\n", i)
		lines += fmt.Sprintf("sleep %.3f\n", float64(intervalMs)/1000.0)
	}
	lines += "sleep " + strconv.Itoa(finalSleepSec) + "\n"
	if err := os.WriteFile(scriptPath, []byte(lines), 0755); err != nil { //nolint:gosec
		t.Fatalf("makeStderrOnlyScript: %v", err)
	}
	return scriptPath
}

// makeStderrThenStdoutScript creates a script that writes `stderrChunks` lines to
// stderr at `stderrIntervalMs` ms intervals, sleeps `gapSleepSec` seconds, then
// writes `stdoutPayload` to stdout and exits.
// Simulates the "thinking via stderr → output dump to stdout" CLI pattern.
func makeStderrThenStdoutScript(t *testing.T, stderrChunks, stderrIntervalMs, gapSleepSec int, stdoutPayload string) string {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "stderr-then-stdout-cli")
	lines := "#!/bin/sh\n"
	for i := 0; i < stderrChunks; i++ {
		lines += fmt.Sprintf("echo chunk%d >&2\n", i)
		lines += fmt.Sprintf("sleep %.3f\n", float64(stderrIntervalMs)/1000.0)
	}
	lines += "sleep " + strconv.Itoa(gapSleepSec) + "\n"
	lines += fmt.Sprintf("echo '%s'\n", escapeForSingleQuotedShell(stdoutPayload))
	if err := os.WriteFile(scriptPath, []byte(lines), 0755); err != nil { //nolint:gosec
		t.Fatalf("makeStderrThenStdoutScript: %v", err)
	}
	return scriptPath
}

// makeChildSpawningScript creates a script that emits `parentOutputChunks` lines
// to stdout at `intervalMs` ms intervals, spawns a background `sleep 60` child,
// then sleeps for `finalSleepSec` seconds.
// Used to verify that stability exit properly kills the entire process group.
func makeChildSpawningScript(t *testing.T, parentOutputChunks, intervalMs, finalSleepSec int) string {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "child-spawning-cli")
	lines := "#!/bin/sh\n"
	for i := 0; i < parentOutputChunks; i++ {
		lines += fmt.Sprintf("echo chunk%d\n", i)
		lines += fmt.Sprintf("sleep %.3f\n", float64(intervalMs)/1000.0)
	}
	lines += "sleep 60 &\n"
	lines += "sleep " + strconv.Itoa(finalSleepSec) + "\n"
	if err := os.WriteFile(scriptPath, []byte(lines), 0755); err != nil { //nolint:gosec
		t.Fatalf("makeChildSpawningScript: %v", err)
	}
	return scriptPath
}

// makeOutputFileOnlyScript creates a script that writes `stderrChunks` lines to
// stderr at `stderrIntervalMs` ms intervals, writes `content` to `outputFile`,
// then sleeps for `finalSleepSec` seconds. Stdout is never written.
// Simulates Codex's -o output-file pattern where output bypasses stdout.
// outputFile and content must not contain single quotes.
func makeOutputFileOnlyScript(t *testing.T, outputFile, content string, stderrChunks, stderrIntervalMs, finalSleepSec int) string {
	return makeOutputFileOnlyScriptWithRepeats(
		t,
		outputFile,
		content,
		stderrChunks,
		stderrIntervalMs,
		1,
		0,
		finalSleepSec,
	)
}

// makeOutputFileOnlyScriptWithRepeats creates a script that writes
// `stderrChunks` lines to stderr at `stderrIntervalMs` ms intervals, writes
// `content` to `outputFile` `fileWrites` times, sleeping `fileWriteIntervalMs`
// ms between writes, then sleeps for `finalSleepSec` seconds. Stdout is never
// written. Simulates CLIs whose output only appears via a file side channel.
// outputFile and content must not contain single quotes.
func makeOutputFileOnlyScriptWithRepeats(
	t *testing.T,
	outputFile, content string,
	stderrChunks, stderrIntervalMs, fileWrites, fileWriteIntervalMs, finalSleepSec int,
) string {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "output-file-only-cli")
	lines := "#!/bin/sh\n"
	for i := 0; i < stderrChunks; i++ {
		lines += fmt.Sprintf("echo progress%d >&2\n", i)
		lines += fmt.Sprintf("sleep %.3f\n", float64(stderrIntervalMs)/1000.0)
	}
	for i := 0; i < fileWrites; i++ {
		var rawContent string
		if i == 0 {
			rawContent = content
		} else {
			rawContent = fmt.Sprintf("%s%d", content, i)
		}
		uniqueContent := escapeForSingleQuotedShell(rawContent)
		lines += fmt.Sprintf("printf '%%s\\n' '%s' > '%s'\n", uniqueContent, escapeForSingleQuotedShell(outputFile))
		if i < fileWrites-1 && fileWriteIntervalMs > 0 {
			lines += fmt.Sprintf("sleep %.3f\n", float64(fileWriteIntervalMs)/1000.0)
		}
	}
	lines += "sleep " + strconv.Itoa(finalSleepSec) + "\n"
	if err := os.WriteFile(scriptPath, []byte(lines), 0755); err != nil { //nolint:gosec
		t.Fatalf("makeOutputFileOnlyScriptWithRepeats: %v", err)
	}
	return scriptPath
}

func TestMakeOutputFileOnlyScript(t *testing.T) {
	outputFile := filepath.Join(t.TempDir(), "output.txt")
	scriptPath := makeOutputFileOnlyScript(t, outputFile, "expected content", 3, 10, 0)

	cmd := exec.Command(scriptPath) //nolint:gosec // Test executes a temp script generated within this test.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("run output-file-only script: %v", err)
	}

	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}

	gotContent, err := os.ReadFile(outputFile) //nolint:gosec // Test reads a temp file created within this test.
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if string(gotContent) != "expected content\n" {
		t.Fatalf("unexpected output file content: %q", string(gotContent))
	}

	if got := stderr.String(); got != "progress0\nprogress1\nprogress2\n" {
		t.Fatalf("unexpected stderr output: %q", got)
	}
}
