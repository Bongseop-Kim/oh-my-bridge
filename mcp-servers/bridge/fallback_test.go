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
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", filepath.Dir(scriptPath)+string(os.PathListSeparator)+origPath)
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
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)
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
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)
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

// TestDelegateTool_UnsupportedCommand_HardError verifies that an unsupported
// command (not "codex" or "gemini") returns a hard error without fallback.
func TestDelegateTool_UnsupportedCommand_HardError(t *testing.T) {
	// Create a fake "not-codex-not-gemini" binary in PATH so detectCLIs marks it as available.
	fakeBin := makeNamedExitScript(t, "not-codex-not-gemini", 0)
	t.Setenv("PATH", filepath.Dir(fakeBin)+string(os.PathListSeparator)+os.Getenv("PATH"))

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
