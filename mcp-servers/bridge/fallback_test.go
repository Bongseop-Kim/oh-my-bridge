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

// makeFakeCodex creates a fake "codex" binary in a temp dir and prepends that
// dir to PATH so exec.LookPath finds it before any real codex installation.
func makeFakeCodex(t *testing.T, exitCode int) {
	t.Helper()
	scriptPath := makeNamedExitScript(t, "codex", exitCode)
	prependDirToPath(t, filepath.Dir(scriptPath))
}

func TestClassifyCliError_Timeout(t *testing.T) {
	err := fmt.Errorf("wrap: %w", ErrTimeout)
	got := classifyCliError(err)
	if got != reasonCLIErrorTimeout {
		t.Errorf("expected %q, got %q", reasonCLIErrorTimeout, got)
	}
}

func TestClassifyCliError_RateLimit_429(t *testing.T) {
	got := classifyCliError(errors.New("exited with code 1: 429 rate limit exceeded"))
	if got != reasonCLIErrorRateLimit {
		t.Errorf("expected %q, got %q", reasonCLIErrorRateLimit, got)
	}
}

func TestClassifyCliError_RateLimit_TooManyRequests(t *testing.T) {
	got := classifyCliError(errors.New("Too Many Requests from upstream"))
	if got != reasonCLIErrorRateLimit {
		t.Errorf("expected %q, got %q", reasonCLIErrorRateLimit, got)
	}
}

func TestClassifyCliError_Crash(t *testing.T) {
	got := classifyCliError(errors.New("segfault"))
	if got != reasonCLIErrorCrash {
		t.Errorf("expected %q, got %q", reasonCLIErrorCrash, got)
	}
}

// TestDelegateTool_CLIError_ReturnsClaude verifies that when a CLI exits with
// a non-zero code the tool returns action="claude" with a cli_error reason
// instead of propagating a hard error.
func TestDelegateTool_CLIError_ReturnsClaude(t *testing.T) {
	// Put a fake "codex" that exits 1 first in PATH so exec.LookPath finds it.
	makeFakeCodex(t, 1)

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OH_MY_BRIDGE_WORKSPACE_ROOT", t.TempDir())

	testCfg := Config{
		Routes: map[string]string{"quick": "fake-model"},
		Models: map[string]ModelDef{
			"fake-model": {Command: "codex", Args: []string{}},
		},
	}
	writeTestConfig(t, home, testCfg)
	saveAndRestoreState(t)
	if err := reloadState(); err != nil {
		t.Fatalf("reloadState: %v", err)
	}

	_, output, err := delegateTool(context.Background(), nil, delegateInput{
		Prompt:   "test prompt",
		Category: "quick",
	})
	if err != nil {
		t.Fatalf("expected no error (fallback), got: %v", err)
	}
	if output.Action != "claude" {
		t.Errorf("expected action=claude, got %q", output.Action)
	}
	if !strings.HasPrefix(output.Reason, "cli_error") {
		t.Errorf("expected reason to start with 'cli_error', got %q", output.Reason)
	}
}

// makeArgsCaptureFakeCodex creates a fake "codex" script that writes all args to a file,
// then returns the args file path for inspection.
func makeArgsCaptureFakeCodex(t *testing.T, argsFile string) {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "codex")
	content := fmt.Sprintf("#!/bin/sh\necho \"$*\" > %s\necho done\n", argsFile)
	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil { //nolint:gosec
		t.Fatalf("makeArgsCaptureFakeCodex: %v", err)
	}
	prependDirToPath(t, dir)
}

// makeArgsCaptureFakeGemini creates a fake "gemini" script that writes all args to a file
// and returns a fake JSON response.
func makeArgsCaptureFakeGemini(t *testing.T, argsFile string) {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "gemini")
	content := fmt.Sprintf("#!/bin/sh\necho \"$*\" > %s\necho '{\"response\": \"done\"}'\n", argsFile)
	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil { //nolint:gosec
		t.Fatalf("makeArgsCaptureFakeGemini: %v", err)
	}
	prependDirToPath(t, dir)
}

// TestDelegateTool_PromptAppend_Codex verifies that category_overrides.prompt_append
// is appended to the prompt when routing to Codex.
func TestDelegateTool_PromptAppend_Codex(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "codex-args.txt")
	makeArgsCaptureFakeCodex(t, argsFile)

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OH_MY_BRIDGE_WORKSPACE_ROOT", t.TempDir())

	testCfg := Config{
		Routes: map[string]string{"deep": "gpt-codex"},
		Models: map[string]ModelDef{
			"gpt-codex": {Command: "codex", Args: []string{}},
		},
		CategoryOverrides: map[string]CategoryOverride{
			"deep": {PromptAppend: "APPEND_MARKER"},
		},
	}
	writeTestConfig(t, home, testCfg)
	saveAndRestoreState(t)
	if err := reloadState(); err != nil {
		t.Fatalf("reloadState: %v", err)
	}

	_, _, err := delegateTool(context.Background(), nil, delegateInput{
		Prompt:   "base prompt",
		Category: "deep",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	argsData, readErr := os.ReadFile(argsFile) //nolint:gosec
	if readErr != nil {
		t.Fatalf("failed to read args file: %v", readErr)
	}
	if !strings.Contains(string(argsData), "APPEND_MARKER") {
		t.Errorf("expected prompt_append 'APPEND_MARKER' in codex args, got: %q", string(argsData))
	}
}

// TestDelegateTool_PromptAppend_Gemini verifies that category_overrides.prompt_append
// is appended to the prompt when routing to Gemini.
func TestDelegateTool_PromptAppend_Gemini(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "gemini-args.txt")
	makeArgsCaptureFakeGemini(t, argsFile)

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OH_MY_BRIDGE_WORKSPACE_ROOT", t.TempDir())

	testCfg := Config{
		Routes: map[string]string{"writing": "gemini-flash"},
		Models: map[string]ModelDef{
			"gemini-flash": {Command: "gemini", Args: []string{}},
		},
		CategoryOverrides: map[string]CategoryOverride{
			"writing": {PromptAppend: "APPEND_MARKER"},
		},
	}
	writeTestConfig(t, home, testCfg)
	saveAndRestoreState(t)
	if err := reloadState(); err != nil {
		t.Fatalf("reloadState: %v", err)
	}

	_, _, err := delegateTool(context.Background(), nil, delegateInput{
		Prompt:   "base prompt",
		Category: "writing",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	argsData, readErr := os.ReadFile(argsFile) //nolint:gosec
	if readErr != nil {
		t.Fatalf("failed to read args file: %v", readErr)
	}
	if !strings.Contains(string(argsData), "APPEND_MARKER") {
		t.Errorf("expected prompt_append 'APPEND_MARKER' in gemini args, got: %q", string(argsData))
	}
}

// makeGeminiStabilityFakeScript creates a fake "gemini" binary in PATH that
// outputs `chunks` lines at `intervalMs` ms intervals to stdout, then sleeps.
// The stdout is non-JSON text; runGemini falls back to raw text on parse failure.
func makeGeminiStabilityFakeScript(t *testing.T, chunks, intervalMs, finalSleepSec int) {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "gemini")
	lines := "#!/bin/sh\n"
	for i := 0; i < chunks; i++ {
		lines += fmt.Sprintf("echo chunk%d\n", i)
		lines += fmt.Sprintf("sleep %.3f\n", float64(intervalMs)/1000.0)
	}
	lines += fmt.Sprintf("sleep %d\n", finalSleepSec)
	if err := os.WriteFile(scriptPath, []byte(lines), 0755); err != nil { //nolint:gosec
		t.Fatalf("makeGeminiStabilityFakeScript: %v", err)
	}
	prependDirToPath(t, dir)
}

// makeGeminiStderrOnlyFakeScript creates a fake "gemini" binary in PATH that
// writes `chunks` lines to stderr at `intervalMs` ms intervals, then sleeps.
// No stdout is produced — simulates Gemini progress-only pattern.
func makeGeminiStderrOnlyFakeScript(t *testing.T, chunks, intervalMs, finalSleepSec int) {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "gemini")
	lines := "#!/bin/sh\n"
	for i := 0; i < chunks; i++ {
		lines += fmt.Sprintf("echo chunk%d >&2\n", i)
		lines += fmt.Sprintf("sleep %.3f\n", float64(intervalMs)/1000.0)
	}
	lines += fmt.Sprintf("sleep %d\n", finalSleepSec)
	if err := os.WriteFile(scriptPath, []byte(lines), 0755); err != nil { //nolint:gosec
		t.Fatalf("makeGeminiStderrOnlyFakeScript: %v", err)
	}
	prependDirToPath(t, dir)
}

// makeCodexOutputFileFakeScript creates a fake "codex" binary in PATH that:
// writes `stderrChunks` progress lines to stderr, optionally writes `fileContent`
// to the -o output file (empty string = no write), then sleeps `finalSleepSec` s.
func makeCodexOutputFileFakeScript(t *testing.T, fileContent string, stderrChunks, intervalMs, finalSleepSec int) {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "codex")
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
		lines += fmt.Sprintf("sleep %.3f\n", float64(intervalMs)/1000.0)
	}
	if fileContent != "" {
		lines += fmt.Sprintf("[ -n \"$outfile\" ] && printf '%%s\\n' '%s' > \"$outfile\"\n", fileContent)
	}
	lines += fmt.Sprintf("sleep %d\n", finalSleepSec)
	if err := os.WriteFile(scriptPath, []byte(lines), 0755); err != nil { //nolint:gosec
		t.Fatalf("makeCodexOutputFileFakeScript: %v", err)
	}
	prependDirToPath(t, dir)
}

// setupDelegateTool configures HOME, OH_MY_BRIDGE_WORKSPACE_ROOT, and the global
// config state for delegateTool integration tests. Returns a cleanup function.
func setupDelegateTool(t *testing.T, cfg Config) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OH_MY_BRIDGE_WORKSPACE_ROOT", t.TempDir())
	writeTestConfig(t, home, cfg)
	saveAndRestoreState(t)
	if err := reloadState(); err != nil {
		t.Fatalf("reloadState: %v", err)
	}
}

// TestDelegateTool_Gemini_StabilityExit_WarningBanner verifies that when Gemini
// is terminated by the stability timeout, delegateTool sets StabilityExit=true
// and prepends the warning banner to the response.
func TestDelegateTool_Gemini_StabilityExit_WarningBanner(t *testing.T) {
	// 3 stdout chunks (non-JSON) → stability exits → warning banner prepended.
	makeGeminiStabilityFakeScript(t, 3, 200, 30)

	setupDelegateTool(t, Config{
		Routes: map[string]string{"quick": "gemini-flash"},
		Models: map[string]ModelDef{
			"gemini-flash": {Command: "gemini", Args: []string{}},
		},
	})

	_, output, err := delegateTool(context.Background(), nil, delegateInput{
		Prompt:               "test prompt",
		Category:             "quick",
		StabilityTimeoutMs:   2000,
		FirstOutputTimeoutMs: 10000,
		MaxTimeoutMs:         60000,
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !output.StabilityExit {
		t.Error("expected StabilityExit = true")
	}
	if !strings.Contains(output.Response, "[WARNING: stability-exit") {
		t.Errorf("expected warning banner in response, got: %q", output.Response)
	}
	t.Logf("Gemini stability warning banner: %q", output.Response[:min(80, len(output.Response))])
}

// TestDelegateTool_Gemini_StabilityExit_EmptyOutput verifies that delegateTool
// handles the stderr-only Gemini pattern: first-output timeout fires, returning
// action="claude" with reason="cli_error_timeout".
func TestDelegateTool_Gemini_StabilityExit_EmptyOutput(t *testing.T) {
	// stderr progress only → empty stdout → first-output timeout → claude fallback.
	makeGeminiStderrOnlyFakeScript(t, 3, 200, 30)

	setupDelegateTool(t, Config{
		Routes: map[string]string{"quick": "gemini-flash"},
		Models: map[string]ModelDef{
			"gemini-flash": {Command: "gemini", Args: []string{}},
		},
	})

	_, output, err := delegateTool(context.Background(), nil, delegateInput{
		Prompt:               "test prompt",
		Category:             "quick",
		StabilityTimeoutMs:   2000,
		FirstOutputTimeoutMs: 10000,
		MaxTimeoutMs:         60000,
	})

	if err != nil {
		t.Fatalf("expected no error (fallback), got: %v", err)
	}
	if output.Action != "claude" {
		t.Errorf("expected action=claude, got %q", output.Action)
	}
	if !strings.HasPrefix(output.Reason, reasonCLIErrorTimeout) {
		t.Errorf("expected reason to start with %q, got %q", reasonCLIErrorTimeout, output.Reason)
	}
	t.Logf("Gemini stderr-only → claude fallback, Reason: %q", output.Reason)
}

// TestDelegateTool_Codex_StabilityExit_OutputFileFallback verifies that when
// Codex writes output to its -o file before stability fires, delegateTool
// returns that file content with the warning banner prepended.
func TestDelegateTool_Codex_StabilityExit_OutputFileFallback(t *testing.T) {
	const fileContent = "codex result via output file"
	makeCodexOutputFileFakeScript(t, fileContent, 2, 300, 30)

	setupDelegateTool(t, Config{
		Routes: map[string]string{"deep": "gpt-codex"},
		Models: map[string]ModelDef{
			"gpt-codex": {Command: "codex", Args: []string{}},
		},
	})

	_, output, err := delegateTool(context.Background(), nil, delegateInput{
		Prompt:               "test prompt",
		Category:             "deep",
		StabilityTimeoutMs:   2000,
		FirstOutputTimeoutMs: 10000,
		MaxTimeoutMs:         60000,
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !output.StabilityExit {
		t.Error("expected StabilityExit = true")
	}
	if !strings.Contains(output.Response, fileContent) {
		t.Errorf("expected output file content %q in response, got: %q", fileContent, output.Response)
	}
	if !strings.Contains(output.Response, "[WARNING: stability-exit") {
		t.Errorf("expected warning banner in response, got: %q", output.Response)
	}
	t.Logf("Codex output-file fallback with banner: %q", output.Response[:min(100, len(output.Response))])
}

// TestDelegateTool_Codex_StabilityExit_AllEmpty verifies that when Codex
// produces nothing on stdout or the output file, the first-output timeout fires
// returning action="claude" with reason="cli_error_timeout".
func TestDelegateTool_Codex_StabilityExit_AllEmpty(t *testing.T) {
	// stderr progress only; no stdout; output file empty → first-output timeout → claude fallback.
	makeCodexOutputFileFakeScript(t, "", 3, 200, 30)

	setupDelegateTool(t, Config{
		Routes: map[string]string{"deep": "gpt-codex"},
		Models: map[string]ModelDef{
			"gpt-codex": {Command: "codex", Args: []string{}},
		},
	})

	_, output, err := delegateTool(context.Background(), nil, delegateInput{
		Prompt:               "test prompt",
		Category:             "deep",
		StabilityTimeoutMs:   2000,
		FirstOutputTimeoutMs: 10000,
		MaxTimeoutMs:         60000,
	})

	if err != nil {
		t.Fatalf("expected no error (fallback), got: %v", err)
	}
	if output.Action != "claude" {
		t.Errorf("expected action=claude, got %q", output.Action)
	}
	if !strings.HasPrefix(output.Reason, reasonCLIErrorTimeout) {
		t.Errorf("expected reason to start with %q, got %q", reasonCLIErrorTimeout, output.Reason)
	}
	t.Logf("Codex all-empty → claude fallback, Reason: %q", output.Reason)
}

// TestDelegateTool_UnsupportedCommand_HardError verifies that an unsupported
// command (not "codex" or "gemini") returns a hard error without fallback.
func TestDelegateTool_UnsupportedCommand_HardError(t *testing.T) {
	// Create a fake "not-codex-not-gemini" binary in PATH so detectCLIs marks it as available.
	fakeBin := makeNamedExitScript(t, "not-codex-not-gemini", 0)
	prependDirToPath(t, filepath.Dir(fakeBin))

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OH_MY_BRIDGE_WORKSPACE_ROOT", t.TempDir())

	testCfg := Config{
		Routes: map[string]string{"quick": "bad-model"},
		Models: map[string]ModelDef{
			"bad-model": {Command: "not-codex-not-gemini", Args: []string{}},
		},
	}
	writeTestConfig(t, home, testCfg)
	saveAndRestoreState(t)
	if err := reloadState(); err != nil {
		t.Fatalf("reloadState: %v", err)
	}

	_, _, err := delegateTool(context.Background(), nil, delegateInput{
		Prompt:   "test prompt",
		Category: "quick",
	})
	if err == nil {
		t.Fatal("expected hard error for unsupported command, got nil")
	}
	if !errors.Is(err, ErrUnsupportedCommand) {
		t.Errorf("expected ErrUnsupportedCommand, got: %v", err)
	}
}
