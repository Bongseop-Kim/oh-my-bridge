package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReloadState_StaleConfig_UsedWhenReloadFails(t *testing.T) {
	// Write a corrupt config so loadConfig returns a JSON parse error.
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir := filepath.Join(home, ".config", "oh-my-bridge")
	if err := os.MkdirAll(configDir, 0750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte("not json"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

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
	// Write a corrupt config so loadConfig returns a JSON parse error.
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir := filepath.Join(home, ".config", "oh-my-bridge")
	if err := os.MkdirAll(configDir, 0750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte("not json"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

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
