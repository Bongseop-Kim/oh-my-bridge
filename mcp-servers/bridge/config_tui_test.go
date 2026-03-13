package main

import "testing"

func TestComputeDiff_noChanges(t *testing.T) {
	original := map[string]string{"quick": "claude", "deep": "gpt-5.3-codex"}
	current := map[string]string{"quick": "claude", "deep": "gpt-5.3-codex"}
	diff := computeDiff(original, current)
	if len(diff) != 0 {
		t.Errorf("expected empty diff, got %v", diff)
	}
}

func TestComputeDiff_oneChange(t *testing.T) {
	original := map[string]string{"quick": "claude", "deep": "gpt-5.3-codex"}
	current := map[string]string{"quick": "claude", "deep": "gemini-3-pro"}
	diff := computeDiff(original, current)
	if len(diff) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diff))
	}
	if diff[0].Category != "deep" || diff[0].From != "gpt-5.3-codex" || diff[0].To != "gemini-3-pro" {
		t.Errorf("unexpected diff entry: %+v", diff[0])
	}
}

func TestComputeDiff_multipleChanges(t *testing.T) {
	original := map[string]string{"quick": "claude", "deep": "gpt-5.3-codex", "writing": "gemini-3-flash"}
	current := map[string]string{"quick": "gpt-5.4", "deep": "gpt-5.3-codex", "writing": "gemini-3-pro"}
	diff := computeDiff(original, current)
	if len(diff) != 2 {
		t.Errorf("expected 2 diffs, got %d: %v", len(diff), diff)
	}
}

func TestBuildDropdownOptions(t *testing.T) {
	models := map[string]ModelDef{
		"gpt-5.4":      {Command: "codex"},
		"gemini-3-pro": {Command: "gemini"},
	}
	opts := buildDropdownOptions(models)
	if opts[0] != "claude" {
		t.Errorf("first option must be claude, got %s", opts[0])
	}
	if len(opts) != 3 { // claude + 2 models
		t.Errorf("expected 3 options, got %d", len(opts))
	}
}
