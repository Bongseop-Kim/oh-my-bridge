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

// TestRunGemini_FirstOutputTimeout verifies that runGemini returns ErrTimeout
// when the Gemini CLI produces no output within FirstOutputTimeoutMs.
// This is the core regression test for issue #10: a hanging CLI must be
// detected early instead of waiting out the full MaxTimeoutMs.
func TestRunGemini_FirstOutputTimeout(t *testing.T) {
	fakeBin := makeSlowScript(t, 30) // hangs for 30s, no output

	start := time.Now()
	_, err := runGemini(context.Background(), runOptions{
		Prompt: "test prompt",
		CWD:    t.TempDir(),
		ModelDef: ModelDef{
			Command: fakeBin,
			Args:    []string{},
		},
		Timeout: timeoutConfig{
			MaxTimeoutMs:         60000,
			FirstOutputTimeoutMs: 1500,
			StabilityTimeoutMs:   2000,
		},
	})
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

// TestRunGemini_StabilityExit verifies that runGemini terminates via the
// stability timeout after output goes quiet, returning StabilityExit: true.
func TestRunGemini_StabilityExit(t *testing.T) {
	// 3 chunks at 200ms intervals, then 30s sleep — stability kicks in after 2s quiet.
	script := makeIncrementalOutputScript(t, 3, 200, 30)

	start := time.Now()
	result, err := runGemini(context.Background(), runOptions{
		Prompt: "test prompt",
		CWD:    t.TempDir(),
		ModelDef: ModelDef{
			Command: script,
			Args:    []string{},
		},
		Timeout: timeoutConfig{
			MaxTimeoutMs:         60000,
			FirstOutputTimeoutMs: 5000,
			StabilityTimeoutMs:   2000,
		},
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !result.StabilityExit {
		t.Error("expected StabilityExit = true, got false")
	}
	// Should finish in about 3s (600ms output + 2s stability window + polling slack).
	if elapsed > 8*time.Second {
		t.Errorf("took too long: %v (want < 8s)", elapsed)
	}
	t.Logf("Gemini stability exit in %v, output: %q", elapsed, result.Text)
}

// TestRunGemini_NaturalExit verifies that a fast-completing Gemini CLI process
// succeeds immediately with StabilityExit = false.
func TestRunGemini_NaturalExit(t *testing.T) {
	script := makeIncrementalOutputScript(t, 3, 100, 0)

	result, err := runGemini(context.Background(), runOptions{
		Prompt: "test prompt",
		CWD:    t.TempDir(),
		ModelDef: ModelDef{
			Command: script,
			Args:    []string{},
		},
		Timeout: timeoutConfig{
			MaxTimeoutMs:         60000,
			FirstOutputTimeoutMs: 5000,
			StabilityTimeoutMs:   5000,
		},
	})

	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if result.StabilityExit {
		t.Error("expected StabilityExit = false for natural exit")
	}
	if result.Text == "" {
		t.Error("expected non-empty output")
	}
	t.Logf("Gemini natural exit, output: %q", result.Text)
}

// TestRunGemini_FastExit verifies that runGemini returns an error immediately
// when the CLI exits with a non-zero code.
func TestRunGemini_FastExit(t *testing.T) {
	fakeBin := makeFastExitScript(t, 1)

	_, err := runGemini(context.Background(), runOptions{
		Prompt: "test prompt",
		CWD:    t.TempDir(),
		ModelDef: ModelDef{
			Command: fakeBin,
			Args:    []string{},
		},
		Timeout: timeoutConfig{
			MaxTimeoutMs:         5000,
			FirstOutputTimeoutMs: 3000,
			StabilityTimeoutMs:   2000,
		},
	})

	if err == nil {
		t.Error("expected error from non-zero exit, got nil")
	}
	t.Logf("Gemini fast-exit returned error immediately: %v", err)
}

// makeArgsEchoScript creates a shell script that echoes all its arguments to stdout.
func makeArgsEchoScript(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "args-echo-cli")
	// Echo all args as JSON-like response so parseGeminiJSON passes through
	content := "#!/bin/sh\necho '{\"response\": \"'\"$*\"'\"}'\n"
	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
		t.Fatalf("makeArgsEchoScript: %v", err)
	}
	return scriptPath
}

// TestRunGemini_ArgsContainApprovalMode verifies that runGemini passes
// --approval-mode=yolo and --output-format json to the CLI.
func TestRunGemini_ArgsContainApprovalMode(t *testing.T) {
	fakeBin := makeArgsEchoScript(t)

	result, err := runGemini(context.Background(), runOptions{
		Prompt: "test prompt",
		CWD:    t.TempDir(),
		ModelDef: ModelDef{
			Command: fakeBin,
			Args:    []string{},
		},
		Timeout: timeoutConfig{
			MaxTimeoutMs:         5000,
			FirstOutputTimeoutMs: 3000,
			StabilityTimeoutMs:   2000,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The script echoes all args; the raw output before parseGeminiJSON would contain the flags.
	// Since the script wraps args in JSON, check that the response contains the flags.
	if !strings.Contains(result.Text, "--approval-mode=yolo") {
		t.Errorf("expected --approval-mode=yolo in args, got: %q", result.Text)
	}
	if !strings.Contains(result.Text, "--output-format json") {
		t.Errorf("expected --output-format json in args, got: %q", result.Text)
	}
}

// TestParseGeminiJSON_Valid verifies that parseGeminiJSON extracts the response field.
func TestParseGeminiJSON_Valid(t *testing.T) {
	raw := `{"session_id":"abc","response":"hello world","stats":{}}`
	got := parseGeminiJSON(raw)
	if got != "hello world" {
		t.Errorf("expected 'hello world', got %q", got)
	}
}

// TestParseGeminiJSON_Invalid verifies that parseGeminiJSON falls back to raw text on parse failure.
func TestParseGeminiJSON_Invalid(t *testing.T) {
	raw := "not valid json"
	got := parseGeminiJSON(raw)
	if got != raw {
		t.Errorf("expected raw fallback %q, got %q", raw, got)
	}
}
