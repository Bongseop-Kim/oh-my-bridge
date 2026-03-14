package main

import (
	"strings"
	"testing"
)

// TestResolveTimeout_Defaults verifies that zero input yields all default values.
func TestResolveTimeout_Defaults(t *testing.T) {
	cfg, err := resolveTimeout(delegateInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxTimeoutMs != defaultMaxTimeoutMs {
		t.Errorf("MaxTimeoutMs = %d, want %d", cfg.MaxTimeoutMs, defaultMaxTimeoutMs)
	}
	if cfg.FirstOutputTimeoutMs != defaultFirstOutputTimeoutMs {
		t.Errorf("FirstOutputTimeoutMs = %d, want %d", cfg.FirstOutputTimeoutMs, defaultFirstOutputTimeoutMs)
	}
	if cfg.StabilityTimeoutMs != defaultStabilityTimeoutMs {
		t.Errorf("StabilityTimeoutMs = %d, want %d", cfg.StabilityTimeoutMs, defaultStabilityTimeoutMs)
	}
}

// TestResolveTimeout_Override verifies that non-zero fields override defaults.
func TestResolveTimeout_Override(t *testing.T) {
	cfg, err := resolveTimeout(delegateInput{
		MaxTimeoutMs:         600000,
		FirstOutputTimeoutMs: 15000,
		StabilityTimeoutMs:   5000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxTimeoutMs != 600000 {
		t.Errorf("MaxTimeoutMs = %d, want 600000", cfg.MaxTimeoutMs)
	}
	if cfg.FirstOutputTimeoutMs != 15000 {
		t.Errorf("FirstOutputTimeoutMs = %d, want 15000", cfg.FirstOutputTimeoutMs)
	}
	if cfg.StabilityTimeoutMs != 5000 {
		t.Errorf("StabilityTimeoutMs = %d, want 5000", cfg.StabilityTimeoutMs)
	}
}

// TestResolveTimeout_NegativeValues verifies that negative values are rejected.
func TestResolveTimeout_NegativeValues(t *testing.T) {
	cases := []delegateInput{
		{MaxTimeoutMs: -1},
		{FirstOutputTimeoutMs: -1},
		{StabilityTimeoutMs: -1},
	}
	for _, input := range cases {
		_, err := resolveTimeout(input)
		if err == nil {
			t.Errorf("expected error for negative timeout, got nil (input=%+v)", input)
		}
	}
}

// TestResolveTimeout_FirstOutputExceedsMax verifies that firstOutput > max is rejected.
func TestResolveTimeout_FirstOutputExceedsMax(t *testing.T) {
	_, err := resolveTimeout(delegateInput{
		MaxTimeoutMs:         1000,
		FirstOutputTimeoutMs: 2000,
	})
	if err == nil {
		t.Fatal("expected error when firstOutputTimeoutMs > maxTimeoutMs, got nil")
	}
	if !strings.Contains(err.Error(), "firstOutputTimeoutMs") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestResolveTimeout_StabilityClampedToMax verifies that stability > max is clamped, not rejected.
func TestResolveTimeout_StabilityClampedToMax(t *testing.T) {
	cfg, err := resolveTimeout(delegateInput{
		MaxTimeoutMs:         1000,
		FirstOutputTimeoutMs: 500,
		StabilityTimeoutMs:   5000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.StabilityTimeoutMs != 1000 {
		t.Errorf("StabilityTimeoutMs = %d, want 1000 (clamped to max)", cfg.StabilityTimeoutMs)
	}
}

// TestResolveTimeout_PartialOverride verifies that only specified fields are overridden.
func TestResolveTimeout_PartialOverride(t *testing.T) {
	cfg, err := resolveTimeout(delegateInput{
		MaxTimeoutMs: 600000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxTimeoutMs != 600000 {
		t.Errorf("MaxTimeoutMs = %d, want 600000", cfg.MaxTimeoutMs)
	}
	if cfg.FirstOutputTimeoutMs != defaultFirstOutputTimeoutMs {
		t.Errorf("FirstOutputTimeoutMs = %d, want default %d", cfg.FirstOutputTimeoutMs, defaultFirstOutputTimeoutMs)
	}
	if cfg.StabilityTimeoutMs != defaultStabilityTimeoutMs {
		t.Errorf("StabilityTimeoutMs = %d, want default %d", cfg.StabilityTimeoutMs, defaultStabilityTimeoutMs)
	}
}
