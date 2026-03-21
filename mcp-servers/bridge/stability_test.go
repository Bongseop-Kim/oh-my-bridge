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

// TestRunCli_StderrOnlyActivity_StabilityExitEmptyStdout verifies that when
// only stderr activity is present (no stdout), the first-output timeout fires
// once stderr goes quiet — returning ErrTimeout instead of a stability exit.
func TestRunCli_StderrOnlyActivity_StabilityExitEmptyStdout(t *testing.T) {
	// 5 stderr chunks at 500ms intervals → 2.5s of activity, then 30s sleep.
	script := makeStderrOnlyScript(t, 5, 500, 30)

	start := time.Now()
	_, err := runCli(context.Background(), cliRequest{
		Command:     script,
		Args:        []string{},
		CWD:         t.TempDir(),
		Timeout:     timeoutConfig{MaxTimeoutMs: 60000, FirstOutputTimeoutMs: 10000, StabilityTimeoutMs: 2000},
		ErrorPrefix: "stderr-only test",
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected ErrTimeout, got nil")
	}
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("expected ErrTimeout, got: %v", err)
	}
	// ~2.5s stderr + 2s aliveQuiet window + firstOutput fires at 10s + polling slack
	if elapsed > 15*time.Second {
		t.Errorf("took too long: %v (want < 15s)", elapsed)
	}
	t.Logf("stderr-only first-output timeout in %v", elapsed)
}

// TestRunCli_StderrThenStdoutBurst_Success verifies the happy path: stderr
// keeps the stability timer alive during "thinking", then a stdout burst causes
// the process to exit naturally with StabilityExit = false.
func TestRunCli_StderrThenStdoutBurst_Success(t *testing.T) {
	const payload = "STDOUT_RESULT"
	// 5 stderr chunks at 200ms, gap=0, then stdout payload → immediate exit.
	script := makeStderrThenStdoutScript(t, 5, 200, 0, payload)

	result, err := runCli(context.Background(), cliRequest{
		Command:     script,
		Args:        []string{},
		CWD:         t.TempDir(),
		Timeout:     timeoutConfig{MaxTimeoutMs: 60000, FirstOutputTimeoutMs: 10000, StabilityTimeoutMs: 2000},
		ErrorPrefix: "stderr-then-stdout test",
	})

	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if result.StabilityExit {
		t.Error("expected StabilityExit = false for natural exit after stdout burst")
	}
	if !strings.Contains(result.Text, payload) {
		t.Errorf("expected %q in output, got: %q", payload, result.Text)
	}
	t.Logf("stderr-then-stdout success, output: %q", result.Text)
}

// TestRunCli_StderrThenLateStdoutBurst_StabilityFiresBeforeBurst verifies that
// when only stderr arrives and then goes quiet, the first-output timeout fires
// before the late stdout burst — returning ErrTimeout.
func TestRunCli_StderrThenLateStdoutBurst_StabilityFiresBeforeBurst(t *testing.T) {
	const payload = "LATE_STDOUT"
	// 3 stderr chunks at 300ms → ~900ms activity, then 10s gap, then stdout.
	// firstOutputTimeoutMs=5000: fires after ~900ms+2s aliveQuiet, once 5s elapsed.
	script := makeStderrThenStdoutScript(t, 3, 300, 10, payload)

	start := time.Now()
	_, err := runCli(context.Background(), cliRequest{
		Command:     script,
		Args:        []string{},
		CWD:         t.TempDir(),
		Timeout:     timeoutConfig{MaxTimeoutMs: 60000, FirstOutputTimeoutMs: 5000, StabilityTimeoutMs: 2000},
		ErrorPrefix: "late-stdout test",
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected ErrTimeout, got nil")
	}
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("expected ErrTimeout, got: %v", err)
	}
	// ~900ms stderr + 2s aliveQuiet window + firstOutput fires at 5s + polling slack
	if elapsed > 8*time.Second {
		t.Errorf("took too long: %v (want < 8s)", elapsed)
	}
	t.Logf("late-stdout first-output timeout in %v", elapsed)
}

// TestRunCli_ChildProcessCleanup_StabilityExit verifies that when stability
// fires, SIGTERM is sent to the entire process group — orphaned background
// children are killed and runCli returns promptly.
func TestRunCli_ChildProcessCleanup_StabilityExit(t *testing.T) {
	// 1 stdout chunk, then spawn background child (sleep 60), then parent sleeps 30s.
	script := makeChildSpawningScript(t, 1, 100, 30)

	start := time.Now()
	result, err := runCli(context.Background(), cliRequest{
		Command:     script,
		Args:        []string{},
		CWD:         t.TempDir(),
		Timeout:     timeoutConfig{MaxTimeoutMs: 10000, FirstOutputTimeoutMs: 5000, StabilityTimeoutMs: 2000},
		ErrorPrefix: "child-cleanup test",
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected success (stability exit), got error: %v", err)
	}
	if !result.StabilityExit {
		t.Error("expected StabilityExit = true")
	}
	// Stability fires ~2s after the single chunk; WaitDelay (2s) covers orphan cleanup.
	if elapsed > 8*time.Second {
		t.Errorf("took too long: %v (want < 8s, orphan child should be killed)", elapsed)
	}
	t.Logf("child-cleanup stability exit in %v, output: %q", elapsed, result.Text)
}

// TestRunCli_OutputFileMtimeResetsStability verifies that output-file mtime
// changes are treated as activity — they bypass the first-output timeout and
// reset the stability timer. Once the file stops being touched, stability fires.
func TestRunCli_OutputFileMtimeResetsStability(t *testing.T) {
	outputFile := filepath.Join(t.TempDir(), "codex-out.txt")
	if err := os.WriteFile(outputFile, []byte(""), 0600); err != nil {
		t.Fatalf("create outputFile: %v", err)
	}

	// Script writes content to outputFile 3 times at 800ms intervals (no stdout/stderr),
	// then sleeps 30s. File mtime+size updates are the only "activity".
	scriptPath := makeOutputFileOnlyScriptWithRepeats(t, outputFile, "data", 0, 0, 3, 800, 30)

	start := time.Now()
	result, err := runCli(context.Background(), cliRequest{
		Command:     scriptPath,
		Args:        []string{},
		CWD:         t.TempDir(),
		Timeout:     timeoutConfig{MaxTimeoutMs: 60000, FirstOutputTimeoutMs: 10000, StabilityTimeoutMs: 2000},
		OutputFile:  outputFile,
		ErrorPrefix: "mtime-resets-stability test",
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected success (stability exit), got error: %v", err)
	}
	if !result.StabilityExit {
		t.Error("expected StabilityExit = true (stability fires after last touch)")
	}
	// 3 touches at 800ms + 2s stability + polling slack; well under firstOutput=10s
	if elapsed > 10*time.Second {
		t.Errorf("took too long: %v (want < 10s)", elapsed)
	}
	t.Logf("mtime-stability exit in %v, output: %q", elapsed, result.Text)
}
