package main

import (
	"testing"
)

// TestDefaultMaxTimeoutMs_Value pins the defaultMaxTimeoutMs constant to 1800000 (30 min).
func TestDefaultMaxTimeoutMs_Value(t *testing.T) {
	const want = 1800000
	if defaultMaxTimeoutMs != want {
		t.Errorf("defaultMaxTimeoutMs = %d, want %d", defaultMaxTimeoutMs, want)
	}
}

// TestDefaultFirstOutputTimeoutMs_Value pins the defaultFirstOutputTimeoutMs constant to 30000.
func TestDefaultFirstOutputTimeoutMs_Value(t *testing.T) {
	const want = 30000
	if defaultFirstOutputTimeoutMs != want {
		t.Errorf("defaultFirstOutputTimeoutMs = %d, want %d", defaultFirstOutputTimeoutMs, want)
	}
}

// TestDefaultStabilityTimeoutMs_Value pins the defaultStabilityTimeoutMs constant to 10000.
func TestDefaultStabilityTimeoutMs_Value(t *testing.T) {
	const want = 10000
	if defaultStabilityTimeoutMs != want {
		t.Errorf("defaultStabilityTimeoutMs = %d, want %d", defaultStabilityTimeoutMs, want)
	}
}
