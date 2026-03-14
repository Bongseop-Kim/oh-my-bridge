package main

import (
	"testing"
)

func TestReloadState_StaleConfig_UsedWhenReloadFails(t *testing.T) {
	// Point HOME to an empty dir so loadConfig returns an error.
	t.Setenv("HOME", t.TempDir())

	// Seed global state with valid routes+models to simulate stale config.
	mu.Lock()
	origCfg := cfg
	origCLIs := availableCLIs
	cfg = Config{
		Routes: map[string]string{"quick": "gpt"},
		Models: map[string]ModelDef{
			"gpt": {Command: "sh"},
		},
	}
	availableCLIs = map[string]bool{"sh": true}
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		cfg = origCfg
		availableCLIs = origCLIs
		mu.Unlock()
	})

	err := reloadState()
	if err != nil {
		t.Fatalf("expected nil (stale config used), got: %v", err)
	}

	mu.Lock()
	routes := cfg.Routes
	mu.Unlock()
	if routes["quick"] != "gpt" {
		t.Errorf("expected stale config to be preserved, got routes: %v", routes)
	}
}

func TestReloadState_EmptyState_ErrorWhenReloadFails(t *testing.T) {
	// Point HOME to an empty dir so loadConfig returns an error.
	t.Setenv("HOME", t.TempDir())

	// Empty global state — no stale config to fall back to.
	mu.Lock()
	origCfg := cfg
	origCLIs := availableCLIs
	cfg = Config{}
	availableCLIs = nil
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		cfg = origCfg
		availableCLIs = origCLIs
		mu.Unlock()
	})

	err := reloadState()
	if err == nil {
		t.Fatal("expected error when state is empty and config reload fails, got nil")
	}
}
