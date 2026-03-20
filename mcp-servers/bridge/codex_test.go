package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeCodexScript creates a fake "codex" binary that:
//   - Reads the output file path from the -o argument passed by runCodex
//   - Writes `stderrChunks` progress lines to stderr at `stderrIntervalMs` ms intervals
//   - Optionally writes `fileContent` to the -o output file (empty string = no write)
//   - Sleeps `finalSleepSec` seconds (triggers stability exit)
//
// Returns the path to the script, which should be used as ModelDef.Command.
func makeCodexScript(t *testing.T, fileContent string, stderrChunks, stderrIntervalMs, finalSleepSec int) string {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "fake-codex")

	// Parse the -o argument that runCodex appends to locate the output file.
	lines := `#!/bin/sh
prev=""
outfile=""
for arg in "$@"; do
    if [ "$prev" = "-o" ]; then
        outfile="$arg"
        break
    fi
    prev="$arg"
done
`
	for i := 0; i < stderrChunks; i++ {
		lines += fmt.Sprintf("echo progress%d >&2\n", i)
		lines += fmt.Sprintf("sleep %.3f\n", float64(stderrIntervalMs)/1000.0)
	}
	if fileContent != "" {
		// Write content to the output file discovered from -o arg.
		// fileContent must not contain single quotes.
		lines += fmt.Sprintf("[ -n \"$outfile\" ] && printf '%%s\\n' '%s' > \"$outfile\"\n", fileContent)
	}
	lines += fmt.Sprintf("sleep %d\n", finalSleepSec)

	if err := os.WriteFile(scriptPath, []byte(lines), 0755); err != nil { //nolint:gosec
		t.Fatalf("makeCodexScript: %v", err)
	}
	return scriptPath
}

// TestRunCodex_StabilityExit_OutputFilePresent verifies that when stability
// fires, runCodex falls back to reading the output file written by the CLI —
// returning its content even though stdout was empty.
func TestRunCodex_StabilityExit_OutputFilePresent(t *testing.T) {
	const fileContent = "codex output via file"
	// Fake codex writes stderr progress → writes fileContent to -o file → sleeps.
	script := makeCodexScript(t, fileContent, 3, 300, 30)

	result, err := runCodex(context.Background(), runOptions{
		Prompt: "test prompt",
		CWD:    t.TempDir(),
		ModelDef: ModelDef{
			Command: script,
			Args:    []string{},
		},
		Timeout: timeoutConfig{MaxTimeoutMs: 60000, FirstOutputTimeoutMs: 10000, StabilityTimeoutMs: 2000},
	})

	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !result.StabilityExit {
		t.Error("expected StabilityExit = true")
	}
	if !strings.Contains(result.Text, fileContent) {
		t.Errorf("expected output file content %q in result, got: %q", fileContent, result.Text)
	}
	t.Logf("output-file fallback, text: %q", result.Text)
}

// TestRunCodex_StabilityExit_OutputFileEmpty verifies that when only stderr
// activity is present (no stdout, no output file data), runCodex returns
// ErrTimeout — the first-output timeout fires once stderr goes quiet.
func TestRunCodex_StabilityExit_OutputFileEmpty(t *testing.T) {
	// Fake codex writes stderr progress only; output file remains empty.
	script := makeCodexScript(t, "", 3, 300, 30)

	_, err := runCodex(context.Background(), runOptions{
		Prompt: "test prompt",
		CWD:    t.TempDir(),
		ModelDef: ModelDef{
			Command: script,
			Args:    []string{},
		},
		Timeout: timeoutConfig{MaxTimeoutMs: 60000, FirstOutputTimeoutMs: 10000, StabilityTimeoutMs: 2000},
	})

	if err == nil {
		t.Fatal("expected ErrTimeout, got nil")
	}
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("expected ErrTimeout, got: %v", err)
	}
	t.Logf("stderr-only codex first-output timeout: %v", err)
}

// TestRunCodex_StabilityExit_OutputFilePartial verifies that stability exit
// returns whatever partial content is in the output file, as-is.
func TestRunCodex_StabilityExit_OutputFilePartial(t *testing.T) {
	const partialContent = "partial codex output"
	script := makeCodexScript(t, partialContent, 2, 300, 30)

	result, err := runCodex(context.Background(), runOptions{
		Prompt: "test prompt",
		CWD:    t.TempDir(),
		ModelDef: ModelDef{
			Command: script,
			Args:    []string{},
		},
		Timeout: timeoutConfig{MaxTimeoutMs: 60000, FirstOutputTimeoutMs: 10000, StabilityTimeoutMs: 2000},
	})

	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !result.StabilityExit {
		t.Error("expected StabilityExit = true")
	}
	if !strings.Contains(result.Text, partialContent) {
		t.Errorf("expected partial content %q in result, got: %q", partialContent, result.Text)
	}
	t.Logf("partial-output fallback, text: %q", result.Text)
}

// TestRunCodex_StderrOnlyActivity_EmptyResult verifies that the Codex-specific
// stderr-only pattern (progress on stderr, nothing on stdout or output file)
// results in ErrTimeout — first-output timeout fires once stderr goes quiet.
func TestRunCodex_StderrOnlyActivity_EmptyResult(t *testing.T) {
	// 5 stderr chunks at 500ms; stdout empty; output file empty.
	script := makeCodexScript(t, "", 5, 500, 30)

	_, err := runCodex(context.Background(), runOptions{
		Prompt: "test prompt",
		CWD:    t.TempDir(),
		ModelDef: ModelDef{
			Command: script,
			Args:    []string{},
		},
		Timeout: timeoutConfig{MaxTimeoutMs: 60000, FirstOutputTimeoutMs: 10000, StabilityTimeoutMs: 2000},
	})

	if err == nil {
		t.Fatal("expected ErrTimeout, got nil")
	}
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("expected ErrTimeout, got: %v", err)
	}
	t.Logf("stderr-only codex first-output timeout: %v", err)
}
