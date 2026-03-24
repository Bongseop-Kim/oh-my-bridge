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
	}, codexClient, nil)
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
// and returns a valid stream-json response.
func makeArgsCaptureFakeGemini(t *testing.T, argsFile string) {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "gemini")
	content := fmt.Sprintf(`#!/bin/sh
echo "$*" > %s
echo '{"type":"init","session_id":"test-session-id","model":"test"}'
echo '{"type":"message","role":"assistant","content":"done"}'
echo '{"type":"result","status":"success","stats":{}}'
`, argsFile)
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
	}, codexClient, nil)
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
	}, nil, nil)
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

// makeGeminiStreamFakeScript creates a fake "gemini" binary in PATH that emits
// a valid stream-json response with the given content, then exits.
func makeGeminiStreamFakeScript(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "gemini")
	lines := "#!/bin/sh\n" +
		"echo '{\"type\":\"init\",\"session_id\":\"test-session-id\",\"model\":\"test\"}'\n" +
		"echo '{\"type\":\"message\",\"role\":\"assistant\",\"content\":\"" + content + "\"}'\n" +
		"echo '{\"type\":\"result\",\"status\":\"success\",\"stats\":{}}'\n"
	if err := os.WriteFile(scriptPath, []byte(lines), 0755); err != nil { //nolint:gosec
		t.Fatalf("makeGeminiStreamFakeScript: %v", err)
	}
	prependDirToPath(t, dir)
}

// makeGeminiStderrOnlyFakeScript creates a fake "gemini" binary in PATH that
// writes `chunks` lines to stderr at `intervalMs` ms intervals, then sleeps.
// No stdout is produced — simulates cold start where init event never arrives.
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

// TestDelegateTool_Gemini_StreamJSON_Success verifies that a valid stream-json
// response is parsed correctly and returned as a successful response without
// a stability-exit warning.
func TestDelegateTool_Gemini_StreamJSON_Success(t *testing.T) {
	makeGeminiStreamFakeScript(t, "hello from gemini")

	setupDelegateTool(t, Config{
		Routes: map[string]string{"quick": "gemini-flash"},
		Models: map[string]ModelDef{
			"gemini-flash": {Command: "gemini", Args: []string{}},
		},
	})

	_, output, err := delegateTool(context.Background(), nil, delegateInput{
		Prompt:               "test prompt",
		Category:             "quick",
		FirstOutputTimeoutMs: 10000,
		MaxTimeoutMs:         60000,
	}, nil, newGeminiSessionStore())

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if output.StabilityExit {
		t.Error("expected StabilityExit = false for stream-json response")
	}
	if !strings.Contains(output.Response, "hello from gemini") {
		t.Errorf("expected response to contain content, got: %q", output.Response)
	}
}

// TestDelegateTool_Gemini_FirstOutputTimeout_Claude verifies that when the init
// event never arrives (stderr-only output), firstOutputTimeout fires and
// delegateTool returns action="claude" with reason="cli_error_timeout".
func TestDelegateTool_Gemini_FirstOutputTimeout_Claude(t *testing.T) {
	// stderr progress only → no init event → firstOutputTimeout → claude fallback.
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
		FirstOutputTimeoutMs: 3000,
		MaxTimeoutMs:         60000,
	}, nil, nil)

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
	}, nil, nil)
	if err == nil {
		t.Fatal("expected hard error for unsupported command, got nil")
	}
	if !errors.Is(err, ErrUnsupportedCommand) {
		t.Errorf("expected ErrUnsupportedCommand, got: %v", err)
	}
}

// makeGeminiArgsAndSessionFakeScript creates a fake "gemini" binary in PATH that:
// - writes all args to argsFile
// - emits a stream-json response with session_id=sessionID
func makeGeminiArgsAndSessionFakeScript(t *testing.T, argsFile, sessionID string) {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "gemini")
	content := fmt.Sprintf(`#!/bin/sh
echo "$*" >> %s
echo '{"type":"init","session_id":"%s","model":"test"}'
echo '{"type":"message","role":"assistant","content":"ok"}'
echo '{"type":"result","status":"success","stats":{}}'
`, argsFile, sessionID)
	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil { //nolint:gosec
		t.Fatalf("makeGeminiArgsAndSessionFakeScript: %v", err)
	}
	prependDirToPath(t, dir)
}

// TestRunGemini_SessionID_StoredAndResumed verifies that after a successful call
// the session_id from the init event is stored, and the next call with the same
// CWD includes --resume <sessionID> in the arguments.
func TestRunGemini_SessionID_StoredAndResumed(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "gemini-args.txt")
	makeGeminiArgsAndSessionFakeScript(t, argsFile, "test-uuid-1234")

	cwd := t.TempDir()
	sessions := newGeminiSessionStore()
	opts := runOptions{
		Prompt:   "hello",
		CWD:      cwd,
		ModelDef: ModelDef{Command: "gemini", Args: []string{}},
		Timeout: timeoutConfig{
			MaxTimeoutMs:         10000,
			FirstOutputTimeoutMs: 5000,
			StabilityTimeoutMs:   2000,
		},
	}

	// First call: no --resume, session stored after success.
	_, err := runGemini(context.Background(), opts, sessions)
	if err != nil {
		t.Fatalf("first call: unexpected error: %v", err)
	}
	if got := sessions.get(cwd); got != "test-uuid-1234" {
		t.Errorf("expected session_id %q stored, got %q", "test-uuid-1234", got)
	}

	// Second call: --resume test-uuid-1234 must appear in captured args.
	_, err = runGemini(context.Background(), opts, sessions)
	if err != nil {
		t.Fatalf("second call: unexpected error: %v", err)
	}
	data, readErr := os.ReadFile(argsFile) //nolint:gosec
	if readErr != nil {
		t.Fatalf("read args file: %v", readErr)
	}
	// argsFile contains two lines (one per call). Second line should have --resume.
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected 2 lines in args file, got %d: %q", len(lines), string(data))
	}
	secondCallArgs := lines[1]
	if !strings.Contains(secondCallArgs, "--resume") || !strings.Contains(secondCallArgs, "test-uuid-1234") {
		t.Errorf("expected --resume test-uuid-1234 in second call args, got: %q", secondCallArgs)
	}
}
