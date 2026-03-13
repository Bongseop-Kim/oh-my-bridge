package main

import (
	"context"
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

