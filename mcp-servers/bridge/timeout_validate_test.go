package main

import (
	"context"
	"strings"
	"testing"
)

// TestMaxTimeoutMs_Value pins the maxTimeoutMs constant to 300000.
// Issue #11: this ceiling prevents large tasks from getting more time,
// even when explicitly requested by the caller.
func TestMaxTimeoutMs_Value(t *testing.T) {
	const want = 300000
	if maxTimeoutMs != want {
		t.Errorf("maxTimeoutMs = %d, want %d", maxTimeoutMs, want)
	}
}

// TestDelegateTool_TimeoutExceedsMax verifies that delegateTool rejects
// timeoutMs values above maxTimeoutMs.
//
// Issue #11: the 300000ms ceiling means large tasks cannot request more time,
// even when explicitly passed by the caller.
func TestDelegateTool_TimeoutExceedsMax(t *testing.T) {
	_, _, err := delegateTool(context.Background(), nil, delegateInput{
		Prompt:    "test prompt",
		Category:  "quick",
		TimeoutMs: maxTimeoutMs + 1,
	})
	if err == nil {
		t.Fatal("expected error for timeoutMs > maxTimeoutMs, got nil")
	}
	if !strings.Contains(err.Error(), "timeoutMs must be between") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestDelegateTool_TimeoutNegative verifies that delegateTool rejects negative timeoutMs.
func TestDelegateTool_TimeoutNegative(t *testing.T) {
	_, _, err := delegateTool(context.Background(), nil, delegateInput{
		Prompt:    "test prompt",
		Category:  "quick",
		TimeoutMs: -1,
	})
	if err == nil {
		t.Fatal("expected error for negative timeoutMs, got nil")
	}
	if !strings.Contains(err.Error(), "timeoutMs must be between") {
		t.Errorf("unexpected error message: %v", err)
	}
}
