package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// makeGeminiStreamScript creates a standalone shell script (full path returned)
// that emits a valid stream-json response with the given content, then exits.
func makeGeminiStreamScript(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "fake-gemini")
	lines := "#!/bin/sh\n" +
		"echo '{\"type\":\"init\",\"session_id\":\"test-sid\",\"model\":\"test\"}'\n" +
		"echo '{\"type\":\"message\",\"role\":\"assistant\",\"content\":\"" + content + "\"}'\n" +
		"echo '{\"type\":\"result\",\"status\":\"success\",\"stats\":{}}'\n"
	if err := os.WriteFile(scriptPath, []byte(lines), 0755); err != nil { //nolint:gosec
		t.Fatalf("makeGeminiStreamScript: %v", err)
	}
	return scriptPath
}

// makeGeminiStreamArgsEchoScript creates a standalone script that emits a valid
// stream-json response where the content contains all CLI arguments.
func makeGeminiStreamArgsEchoScript(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "fake-gemini")
	content := `#!/bin/sh
echo '{"type":"init","session_id":"test-sid","model":"test"}'
printf '{"type":"message","role":"assistant","content":"%s"}\n' "$*"
echo '{"type":"result","status":"success","stats":{}}'
`
	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil { //nolint:gosec
		t.Fatalf("makeGeminiStreamArgsEchoScript: %v", err)
	}
	return scriptPath
}

func geminiOpts(t *testing.T, cmd string, to timeoutConfig) runOptions {
	t.Helper()
	return runOptions{
		Prompt:   "test prompt",
		CWD:      t.TempDir(),
		ModelDef: ModelDef{Command: cmd, Args: []string{}},
		Timeout:  to,
	}
}

// TestRunGemini_FirstOutputTimeout verifies that runGemini returns ErrTimeout
// when the process produces no stdout (no init event) within FirstOutputTimeoutMs.
func TestRunGemini_FirstOutputTimeout(t *testing.T) {
	fakeBin := makeSlowScript(t, 30) // hangs for 30s, no output

	start := time.Now()
	_, err := runGemini(context.Background(), geminiOpts(t, fakeBin, timeoutConfig{
		MaxTimeoutMs:         60000,
		FirstOutputTimeoutMs: 1500,
		StabilityTimeoutMs:   2000,
	}), nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected ErrTimeout, got nil")
	}
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("expected ErrTimeout, got: %v", err)
	}
	// Should timeout in ~1.5–2.5s (1500ms + up to 1s polling slack).
	if elapsed > 4*time.Second {
		t.Errorf("took too long: %v (want < 4s)", elapsed)
	}
	t.Logf("Gemini first-output timeout in %v", elapsed)
}

// TestRunGemini_StreamJSON_Success verifies that a valid stream-json response
// is parsed and returned as a successful result without StabilityExit.
func TestRunGemini_StreamJSON_Success(t *testing.T) {
	script := makeGeminiStreamScript(t, "hello stream")

	result, err := runGemini(context.Background(), geminiOpts(t, script, timeoutConfig{
		MaxTimeoutMs:         10000,
		FirstOutputTimeoutMs: 5000,
		StabilityTimeoutMs:   2000,
	}), nil)

	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if result.StabilityExit {
		t.Error("expected StabilityExit = false for stream-json response")
	}
	if result.Text != "hello stream" {
		t.Errorf("expected %q, got %q", "hello stream", result.Text)
	}
}

// TestRunGemini_FastExit verifies that runGemini returns an error immediately
// when the CLI exits with a non-zero code.
func TestRunGemini_FastExit(t *testing.T) {
	fakeBin := makeFastExitScript(t, 1)

	_, err := runGemini(context.Background(), geminiOpts(t, fakeBin, timeoutConfig{
		MaxTimeoutMs:         5000,
		FirstOutputTimeoutMs: 3000,
		StabilityTimeoutMs:   2000,
	}), nil)

	if err == nil {
		t.Error("expected error from non-zero exit, got nil")
	}
	t.Logf("Gemini fast-exit returned error immediately: %v", err)
}

// TestRunGemini_ArgsContainFlags verifies that runGemini passes --approval-mode=yolo
// and -o stream-json to the CLI.
func TestRunGemini_ArgsContainFlags(t *testing.T) {
	script := makeGeminiStreamArgsEchoScript(t)

	result, err := runGemini(context.Background(), geminiOpts(t, script, timeoutConfig{
		MaxTimeoutMs:         10000,
		FirstOutputTimeoutMs: 5000,
		StabilityTimeoutMs:   2000,
	}), nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Text, "--approval-mode=yolo") {
		t.Errorf("expected --approval-mode=yolo in args, got: %q", result.Text)
	}
	if !strings.Contains(result.Text, "-o") || !strings.Contains(result.Text, "stream-json") {
		t.Errorf("expected -o stream-json in args, got: %q", result.Text)
	}
}

// TestRunGemini_StderrOnlyActivity_FirstOutputTimeout verifies that stderr-only
// output (no init event on stdout) results in firstOutputTimeout ErrTimeout.
func TestRunGemini_StderrOnlyActivity_FirstOutputTimeout(t *testing.T) {
	script := makeStderrOnlyScript(t, 5, 300, 30)

	_, err := runGemini(context.Background(), geminiOpts(t, script, timeoutConfig{
		MaxTimeoutMs:         60000,
		FirstOutputTimeoutMs: 5000,
		StabilityTimeoutMs:   2000,
	}), nil)

	if err == nil {
		t.Fatal("expected ErrTimeout, got nil")
	}
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("expected ErrTimeout, got: %v", err)
	}
	t.Logf("stderr-only gemini first-output timeout: %v", err)
}

// TestRunGemini_MaxTimeout verifies that runGemini returns ErrTimeout when the
// process exceeds MaxTimeoutMs even while writing output.
func TestRunGemini_MaxTimeout(t *testing.T) {
	// Script emits an init event then loops forever without a result event.
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "endless-gemini")
	script := "#!/bin/sh\necho '{\"type\":\"init\",\"session_id\":\"x\",\"model\":\"test\"}'\nwhile true; do sleep 0.5; done\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil { //nolint:gosec
		t.Fatalf("write script: %v", err)
	}

	start := time.Now()
	_, err := runGemini(context.Background(), geminiOpts(t, scriptPath, timeoutConfig{
		MaxTimeoutMs:         2000,
		FirstOutputTimeoutMs: 10000,
		StabilityTimeoutMs:   10000,
	}), nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected ErrTimeout, got nil")
	}
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("expected ErrTimeout, got: %v", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("took too long: %v (want < 5s)", elapsed)
	}
	t.Logf("Gemini max-timeout in %v: %v", elapsed, err)
}

// TestRunGemini_SessionID_Stored verifies that the session_id from the init event
// is stored in the session store after a successful call.
func TestRunGemini_SessionID_Stored(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "fake-gemini")
	script := "#!/bin/sh\n" +
		"echo '{\"type\":\"init\",\"session_id\":\"my-uuid-42\",\"model\":\"test\"}'\n" +
		"echo '{\"type\":\"message\",\"role\":\"assistant\",\"content\":\"ok\"}'\n" +
		"echo '{\"type\":\"result\",\"status\":\"success\",\"stats\":{}}'\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil { //nolint:gosec
		t.Fatalf("write script: %v", err)
	}

	sessions := newGeminiSessionStore()
	cwd := t.TempDir()

	_, err := runGemini(context.Background(), runOptions{
		Prompt:   "hello",
		CWD:      cwd,
		ModelDef: ModelDef{Command: scriptPath, Args: []string{}},
		Timeout:  timeoutConfig{MaxTimeoutMs: 10000, FirstOutputTimeoutMs: 5000, StabilityTimeoutMs: 2000},
	}, sessions)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := sessions.get(cwd); got != "my-uuid-42" {
		t.Errorf("expected session_id %q stored, got %q", "my-uuid-42", got)
	}
}

// TestRunGemini_ErrorClearsSession verifies that a failed call clears any stored
// session for that CWD to prevent reuse of a stale session.
func TestRunGemini_ErrorClearsSession(t *testing.T) {
	sessions := newGeminiSessionStore()
	cwd := t.TempDir()
	sessions.set(cwd, "stale-session")

	fakeBin := makeFastExitScript(t, 1) // exits non-zero

	_, err := runGemini(context.Background(), runOptions{
		Prompt:   "hello",
		CWD:      cwd,
		ModelDef: ModelDef{Command: fakeBin, Args: []string{}},
		Timeout:  timeoutConfig{MaxTimeoutMs: 5000, FirstOutputTimeoutMs: 3000, StabilityTimeoutMs: 2000},
	}, sessions)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := sessions.get(cwd); got != "" {
		t.Errorf("expected session cleared after error, got %q", got)
	}
}
