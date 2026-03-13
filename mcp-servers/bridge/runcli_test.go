package main

import (
	"context"
	"errors"
	"testing"
)

// TestRunCli_FastExit verifies that a CLI exiting immediately with a non-zero
// code returns an error without waiting for the timeout.
func TestRunCli_FastExit(t *testing.T) {
	fakeCLI := makeFastExitScript(t, 1)

	_, err := runCli(context.Background(), cliRequest{
		Command:     fakeCLI,
		Args:        []string{},
		CWD:         t.TempDir(),
		TimeoutMs:   5000,
		ErrorPrefix: "Codex CLI",
	})

	if err == nil {
		t.Error("expected error from non-zero exit, got nil")
	}
	t.Logf("fast-exit returned error immediately: %v", err)
}

// TestRunCli_Timeout verifies that runCli returns ErrTimeout when the CLI
// does not finish within the configured TimeoutMs.
//
// Root cause of issue #11: all tasks share a fixed 5-minute ceiling, so
// runCli must honour whatever TimeoutMs the caller provides — not a global
// constant.
func TestRunCli_Timeout(t *testing.T) {
	slowCLI := makeSlowScript(t, 10)

	_, err := runCli(context.Background(), cliRequest{
		Command:     slowCLI,
		Args:        []string{},
		CWD:         t.TempDir(),
		TimeoutMs:   50,
		ErrorPrefix: "slow CLI",
	})

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("expected ErrTimeout, got: %v", err)
	}
}
