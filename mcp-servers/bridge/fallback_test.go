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

// TestDelegateTool_CLIError_ReturnsClaude verifies that when the codex MCP server
// returns an error response, delegateTool returns action="claude" with a cli_error
// reason instead of propagating a hard error.
func TestDelegateTool_CLIError_ReturnsClaude(t *testing.T) {
	// fake codex mcp server: IsError=true 응답 반환
	serverBin := newFakeCodexMCPServer(t, "model error occurred", "", true)
	codexClient := newCodexMCPClient(serverBin)

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OH_MY_BRIDGE_WORKSPACE_ROOT", t.TempDir())

	testCfg := Config{
		Routes: map[string]string{"quick": "fake-model"},
		Models: map[string]ModelDef{
			"fake-model": {Command: cmdCodex, Args: []string{}},
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
	}, codexClient)
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
// is appended to the prompt when routing to Codex via MCP.
func TestDelegateTool_PromptAppend_Codex(t *testing.T) {
	captureFile := filepath.Join(t.TempDir(), "captured-prompt.txt")
	serverBin := newFakeCodexMCPServer(t, "ok", captureFile, false)
	codexClient := newCodexMCPClient(serverBin)

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OH_MY_BRIDGE_WORKSPACE_ROOT", t.TempDir())

	testCfg := Config{
		Routes: map[string]string{"deep": "gpt-codex"},
		Models: map[string]ModelDef{
			"gpt-codex": {Command: cmdCodex, Args: []string{}},
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
	}, codexClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, readErr := os.ReadFile(captureFile) //nolint:gosec
	if readErr != nil {
		t.Fatalf("failed to read capture file: %v", readErr)
	}
	if !strings.Contains(string(data), "APPEND_MARKER") {
		t.Errorf("expected prompt_append 'APPEND_MARKER' in captured prompt, got: %q", string(data))
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
	}, nil)
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
	}, nil)

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
	}, nil)

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
	}, nil)
	if err == nil {
		t.Fatal("expected hard error for unsupported command, got nil")
	}
	if !errors.Is(err, ErrUnsupportedCommand) {
		t.Errorf("expected ErrUnsupportedCommand, got: %v", err)
	}
}
