package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// makeFakeCodex creates a fake "codex" binary in a temp dir, prepends that dir
// to PATH, and returns the dir. The binary exits with exitCode immediately.
func makeFakeCodex(t *testing.T, exitCode int) {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "codex")
	content := "#!/bin/sh\nexit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
		t.Fatalf("makeFakeCodex: %v", err)
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)
}

func TestClassifyCliError_Timeout(t *testing.T) {
	err := fmt.Errorf("wrap: %w", ErrTimeout)
	got := classifyCliError(err, "")
	if got != "cli_error_timeout" {
		t.Errorf("expected cli_error_timeout, got %q", got)
	}
}

func TestClassifyCliError_RateLimit_429(t *testing.T) {
	got := classifyCliError(errors.New("some error"), "exited with code 1: 429 rate limit exceeded")
	if got != "cli_error_rate_limit" {
		t.Errorf("expected cli_error_rate_limit, got %q", got)
	}
}

func TestClassifyCliError_RateLimit_TooManyRequests(t *testing.T) {
	got := classifyCliError(errors.New("some error"), "Too Many Requests")
	if got != "cli_error_rate_limit" {
		t.Errorf("expected cli_error_rate_limit, got %q", got)
	}
}

func TestClassifyCliError_Crash(t *testing.T) {
	got := classifyCliError(errors.New("some error"), "segfault")
	if got != "cli_error_crash" {
		t.Errorf("expected cli_error_crash, got %q", got)
	}
}

// TestDelegateTool_CLIError_ReturnsClaude verifies that when a CLI exits with
// a non-zero code the tool returns action="claude" with a cli_error reason
// instead of propagating a hard error.
func TestDelegateTool_CLIError_ReturnsClaude(t *testing.T) {
	// Put a fake "codex" that exits 1 first in PATH so exec.LookPath finds it.
	makeFakeCodex(t, 1)

	// Point HOME to an empty dir so reloadState uses stale config.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OH_MY_BRIDGE_WORKSPACE_ROOT", t.TempDir())

	mu.Lock()
	origCfg := cfg
	origCLIs := availableCLIs
	cfg = Config{
		Routes: map[string]string{"quick": "fake-model"},
		Models: map[string]ModelDef{
			"fake-model": {Command: "codex", Args: []string{}},
		},
	}
	availableCLIs = map[string]bool{"codex": true}
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		cfg = origCfg
		availableCLIs = origCLIs
		mu.Unlock()
	})

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
	if !strings.Contains(output.Reason, "cli_error") {
		t.Errorf("expected reason to contain 'cli_error', got %q", output.Reason)
	}
}

// TestDelegateTool_UnsupportedCommand_HardError verifies that an unsupported
// command (not "codex" or "gemini") returns a hard error without fallback.
func TestDelegateTool_UnsupportedCommand_HardError(t *testing.T) {
	// Point HOME to empty dir so reloadState uses stale config.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OH_MY_BRIDGE_WORKSPACE_ROOT", t.TempDir())

	mu.Lock()
	origCfg := cfg
	origCLIs := availableCLIs
	cfg = Config{
		Routes: map[string]string{"quick": "bad-model"},
		Models: map[string]ModelDef{
			"bad-model": {Command: "not-codex-not-gemini", Args: []string{}},
		},
	}
	availableCLIs = map[string]bool{"not-codex-not-gemini": true}
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		cfg = origCfg
		availableCLIs = origCLIs
		mu.Unlock()
	})

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
