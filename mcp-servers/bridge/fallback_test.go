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
	if !strings.HasPrefix(output.Reason, "cli_error") {
		t.Errorf("expected reason to start with 'cli_error', got %q", output.Reason)
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
