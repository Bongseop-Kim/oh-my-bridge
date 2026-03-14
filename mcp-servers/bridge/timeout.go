package main

import "errors"

// resolveTimeout builds a timeoutConfig from delegateInput, applying defaults
// for zero values and validating constraints.
func resolveTimeout(input delegateInput) (timeoutConfig, error) {
	if input.MaxTimeoutMs < 0 || input.FirstOutputTimeoutMs < 0 || input.StabilityTimeoutMs < 0 {
		return timeoutConfig{}, errors.New("timeout values must be non-negative")
	}
	tc := timeoutConfig{
		MaxTimeoutMs:         defaultMaxTimeoutMs,
		FirstOutputTimeoutMs: defaultFirstOutputTimeoutMs,
		StabilityTimeoutMs:   defaultStabilityTimeoutMs,
	}
	if input.MaxTimeoutMs != 0 {
		tc.MaxTimeoutMs = input.MaxTimeoutMs
	}
	if input.FirstOutputTimeoutMs != 0 {
		tc.FirstOutputTimeoutMs = input.FirstOutputTimeoutMs
	}
	if input.StabilityTimeoutMs != 0 {
		tc.StabilityTimeoutMs = input.StabilityTimeoutMs
	}
	if tc.FirstOutputTimeoutMs > tc.MaxTimeoutMs {
		return timeoutConfig{}, errors.New("firstOutputTimeoutMs must not exceed maxTimeoutMs")
	}
	if tc.StabilityTimeoutMs > tc.MaxTimeoutMs {
		tc.StabilityTimeoutMs = tc.MaxTimeoutMs
	}
	return tc, nil
}
