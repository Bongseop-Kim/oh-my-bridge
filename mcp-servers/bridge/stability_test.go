package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestRunCli_StabilityExit verifies that runCli terminates via the stability
// timeout after output goes quiet, returning StabilityExit: true.
func TestRunCli_StabilityExit(t *testing.T) {
	// 3 chunks at 200ms intervals, then 30s sleep — stability kicks in after 2s quiet.
	script := makeIncrementalOutputScript(t, 3, 200, 30)

	start := time.Now()
	result, err := runCli(context.Background(), cliRequest{
		Command:     script,
		Args:        []string{},
		CWD:         t.TempDir(),
		Timeout:     timeoutConfig{MaxTimeoutMs: 60000, FirstOutputTimeoutMs: 5000, StabilityTimeoutMs: 2000},
		ErrorPrefix: "stability test",
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
	t.Logf("stability exit in %v, output: %q", elapsed, result.Text)
}

// TestRunCli_FirstOutputTimeout verifies that runCli returns ErrTimeout when
// the process produces no output within FirstOutputTimeoutMs.
func TestRunCli_FirstOutputTimeout(t *testing.T) {
	script := makeSlowScript(t, 30)

	start := time.Now()
	_, err := runCli(context.Background(), cliRequest{
		Command:     script,
		Args:        []string{},
		CWD:         t.TempDir(),
		Timeout:     timeoutConfig{MaxTimeoutMs: 60000, FirstOutputTimeoutMs: 1500, StabilityTimeoutMs: 2000},
		ErrorPrefix: "first-output test",
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
	t.Logf("first-output timeout in %v", elapsed)
}

// TestRunCli_FirstOutputTimeout_OutputArrives verifies that when output arrives
// before firstOutputTimeoutMs, the stability logic takes over and succeeds.
func TestRunCli_FirstOutputTimeout_OutputArrives(t *testing.T) {
	// Sleep 1s (within firstOutput window of 5s), emit output, then sleep 30s.
	script := makeIncrementalOutputScript(t, 1, 1000, 30)

	start := time.Now()
	result, err := runCli(context.Background(), cliRequest{
		Command:     script,
		Args:        []string{},
		CWD:         t.TempDir(),
		Timeout:     timeoutConfig{MaxTimeoutMs: 60000, FirstOutputTimeoutMs: 5000, StabilityTimeoutMs: 2000},
		ErrorPrefix: "first-output-arrives test",
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected success after output arrives, got: %v", err)
	}
	if !result.StabilityExit {
		t.Error("expected StabilityExit = true once stability kicks in")
	}
	if elapsed > 10*time.Second {
		t.Errorf("took too long: %v (want < 10s)", elapsed)
	}
	t.Logf("completed in %v via stability exit", elapsed)
}

// TestRunCli_MaxTimeoutCeiling verifies that MaxTimeoutMs is the hard ceiling
// even when there is continuous output.
func TestRunCli_MaxTimeoutCeiling(t *testing.T) {
	// Infinite output every 500ms — max timeout must fire first.
	script := makeIncrementalOutputScript(t, 1000, 500, 0)

	start := time.Now()
	_, err := runCli(context.Background(), cliRequest{
		Command:     script,
		Args:        []string{},
		CWD:         t.TempDir(),
		Timeout:     timeoutConfig{MaxTimeoutMs: 2000, FirstOutputTimeoutMs: 1000, StabilityTimeoutMs: 1000},
		ErrorPrefix: "max-timeout test",
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected ErrTimeout from max ceiling, got nil")
	}
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("expected ErrTimeout, got: %v", err)
	}
	// Should timeout in ~2–3s.
	if elapsed > 5*time.Second {
		t.Errorf("took too long: %v (want < 5s)", elapsed)
	}
	t.Logf("max-timeout fired in %v", elapsed)
}

// TestRunCli_NaturalExit verifies that a fast-completing process succeeds
// immediately with StabilityExit = false.
func TestRunCli_NaturalExit(t *testing.T) {
	script := makeIncrementalOutputScript(t, 3, 100, 0)

	result, err := runCli(context.Background(), cliRequest{
		Command:     script,
		Args:        []string{},
		CWD:         t.TempDir(),
		Timeout:     timeoutConfig{MaxTimeoutMs: 60000, FirstOutputTimeoutMs: 5000, StabilityTimeoutMs: 5000},
		ErrorPrefix: "natural-exit test",
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
	t.Logf("natural exit, output: %q", result.Text)
}
