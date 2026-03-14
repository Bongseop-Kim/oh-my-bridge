package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureConfig_NewFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	if err := ensureConfig(configPath); err != nil {
		t.Fatalf("ensureConfig: %v", err)
	}

	data, err := os.ReadFile(configPath) //nolint:gosec
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	for _, cat := range defaultCategories {
		if _, ok := c.Routes[cat]; !ok {
			t.Errorf("missing default route for category %q", cat)
		}
	}
	if len(c.Models) < 7 {
		t.Errorf("expected >= 7 models, got %d", len(c.Models))
	}
}

func TestEnsureConfig_MergeExisting(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	partial := `{"routes":{"quick":"claude","custom-cat":"codex"},"models":{"custom-model":{"command":"codex","args":[]}}}`
	if err := os.WriteFile(configPath, []byte(partial), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := ensureConfig(configPath); err != nil {
		t.Fatalf("ensureConfig: %v", err)
	}

	data, err := os.ReadFile(configPath) //nolint:gosec
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if c.Routes["quick"] != "claude" {
		t.Errorf("existing route 'quick' should be preserved as 'claude', got %q", c.Routes["quick"])
	}
	if _, ok := c.Routes["custom-cat"]; !ok {
		t.Error("user route 'custom-cat' should be preserved")
	}
	if _, ok := c.Routes["ultrabrain"]; !ok {
		t.Error("missing default route 'ultrabrain' should have been merged")
	}
	if _, ok := c.Models["custom-model"]; !ok {
		t.Error("user model 'custom-model' should be preserved")
	}
}

func TestLoadConfig_AutoCreate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Config does not exist yet — loadConfig should auto-create it via ensureConfig.
	c, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	// Also verify the file was written to the expected path.
	configPath := filepath.Join(dir, ".config", "oh-my-bridge", "config.json")
	data, err := os.ReadFile(configPath) //nolint:gosec
	if err != nil {
		t.Fatalf("ReadFile after loadConfig: %v", err)
	}
	var cFromFile Config
	if err := json.Unmarshal(data, &cFromFile); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	for _, cat := range defaultCategories {
		if _, ok := c.Routes[cat]; !ok {
			t.Errorf("auto-created config missing default route for category %q", cat)
		}
		if _, ok := cFromFile.Routes[cat]; !ok {
			t.Errorf("config file missing default route for category %q", cat)
		}
	}
	if len(c.Models) < 7 {
		t.Errorf("expected >= 7 models in auto-created config, got %d", len(c.Models))
	}
}

func TestInstallSkills_Overwrite(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills")

	if err := installSkills(skillDir, []byte("v1 content"), []byte("v1 slim")); err != nil {
		t.Fatalf("installSkills v1: %v", err)
	}
	if err := installSkills(skillDir, []byte("v2 content"), []byte("v2 slim")); err != nil {
		t.Fatalf("installSkills v2: %v", err)
	}

	skill, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md")) //nolint:gosec
	if err != nil {
		t.Fatalf("ReadFile SKILL.md: %v", err)
	}
	if string(skill) != "v2 content" {
		t.Errorf("SKILL.md: got %q, want %q", string(skill), "v2 content")
	}

	slim, err := os.ReadFile(filepath.Join(skillDir, "code-routing-slim.md")) //nolint:gosec
	if err != nil {
		t.Fatalf("ReadFile slim: %v", err)
	}
	if string(slim) != "v2 slim" {
		t.Errorf("code-routing-slim.md: got %q, want %q", string(slim), "v2 slim")
	}
}

func TestInstallHook_DuplicateRemoval(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, "hooks", "subagent-code-routing.sh")
	settingsPath := filepath.Join(dir, "settings.json")
	hookSH := []byte("#!/bin/bash\necho hi")

	if err := installHook(hookPath, settingsPath, hookSH); err != nil {
		t.Fatalf("installHook first: %v", err)
	}
	if err := installHook(hookPath, settingsPath, hookSH); err != nil {
		t.Fatalf("installHook second: %v", err)
	}

	data, err := os.ReadFile(settingsPath) //nolint:gosec
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var s map[string]any
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	hooks := s["hooks"].(map[string]any)
	groups := hooks["SubagentStart"].([]any)

	count := 0
	for _, gRaw := range groups {
		g := gRaw.(map[string]any)
		inners := g["hooks"].([]any)
		for _, iRaw := range inners {
			ih := iRaw.(map[string]any)
			if ih["command"] == hookPath {
				count++
			}
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 hook entry for %s, got %d", hookPath, count)
	}
}

func TestInstallHook_InvalidSettingsJSON(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, "hook.sh")
	settingsPath := filepath.Join(dir, "settings.json")

	if err := os.WriteFile(settingsPath, []byte("not json"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := installHook(hookPath, settingsPath, []byte("#!/bin/bash"))
	if err == nil {
		t.Fatal("expected error for invalid settings JSON, got nil")
	}
	if !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("error should mention 'not valid JSON', got: %v", err)
	}
}
