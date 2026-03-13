package main

import (
	"testing"
)

func TestValidateConfig_valid(t *testing.T) {
	c := Config{
		Routes: map[string]string{
			"visual-engineering": "gemini-3-pro",
			"ultrabrain":         "gpt-5.3-codex",
			"deep":               "gpt-5.3-codex",
			"artistry":           "gemini-3-pro",
			"quick":              "claude",
			"writing":            "gemini-3-flash",
			"unspecified-high":   "gpt-5.4",
			"unspecified-low":    "claude",
		},
		Models: map[string]ModelDef{
			"gemini-3-pro":   {Command: "gemini", Args: []string{"-m", "gemini-3-pro"}},
			"gpt-5.3-codex":  {Command: "codex", Args: []string{"exec", "-m", "gpt-5.3-codex"}},
			"gemini-3-flash": {Command: "gemini", Args: []string{"-m", "gemini-3-flash"}},
			"gpt-5.4":        {Command: "codex", Args: []string{"exec", "-m", "gpt-5.4"}},
		},
	}
	errs := validateConfigRules(c)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateConfig_missingModel(t *testing.T) {
	c := Config{
		Routes: map[string]string{
			"visual-engineering": "nonexistent-model",
			"ultrabrain":         "claude",
			"deep":               "claude",
			"artistry":           "claude",
			"quick":              "claude",
			"writing":            "claude",
			"unspecified-high":   "claude",
			"unspecified-low":    "claude",
		},
		Models: map[string]ModelDef{},
	}
	errs := validateConfigRules(c)
	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d: %v", len(errs), errs)
	}
}

func TestValidateConfig_nilSections(t *testing.T) {
	errs := validateConfigRules(Config{})
	if len(errs) != 2 {
		t.Errorf("expected 2 errors (nil routes + nil models), got %d: %v", len(errs), errs)
	}
}

func TestCLIStatus_builtin(t *testing.T) {
	clis := map[string]bool{"gemini": true}
	models := map[string]ModelDef{
		"gemini-3-pro": {Command: "gemini"},
	}
	status := cliStatusFor("claude", models, clis)
	if status.Kind != cliBuiltin {
		t.Errorf("expected builtin, got %v", status.Kind)
	}
}

func TestCLIStatus_available(t *testing.T) {
	clis := map[string]bool{"gemini": true}
	models := map[string]ModelDef{
		"gemini-3-pro": {Command: "gemini"},
	}
	status := cliStatusFor("gemini-3-pro", models, clis)
	if status.Kind != cliAvailable {
		t.Errorf("expected available, got %v", status.Kind)
	}
	if status.Command != "gemini" {
		t.Errorf("expected command gemini, got %s", status.Command)
	}
}

func TestCLIStatus_missing(t *testing.T) {
	clis := map[string]bool{"codex": false}
	models := map[string]ModelDef{
		"gpt-5.4": {Command: "codex"},
	}
	status := cliStatusFor("gpt-5.4", models, clis)
	if status.Kind != cliMissing {
		t.Errorf("expected missing, got %v", status.Kind)
	}
}

func TestValidateConfig_CategoryOverrideOrphan(t *testing.T) {
	c := Config{
		Routes: map[string]string{
			"deep": "gpt-5.3-codex",
		},
		Models: map[string]ModelDef{
			"gpt-5.3-codex": {Command: "codex", Args: []string{}},
		},
		CategoryOverrides: map[string]CategoryOverride{
			"nonexistent-category": {ReasoningEffort: "high"},
		},
	}
	errs := validateConfigRules(c)
	if len(errs) != 1 {
		t.Fatalf("expected 1 warning error, got %d: %v", len(errs), errs)
	}
	if !errs[0].Warn {
		t.Errorf("expected Warn=true for orphan category_override, got Warn=false")
	}
}

func TestValidateConfig_CategoryOverrideValid(t *testing.T) {
	c := Config{
		Routes: map[string]string{
			"deep":    "gpt-5.3-codex",
			"writing": "gemini-3-flash",
		},
		Models: map[string]ModelDef{
			"gpt-5.3-codex":  {Command: "codex", Args: []string{}},
			"gemini-3-flash": {Command: "gemini", Args: []string{}},
		},
		CategoryOverrides: map[string]CategoryOverride{
			"deep":    {ReasoningEffort: "high"},
			"writing": {PromptAppend: "한국어로 작성하라."},
		},
	}
	errs := validateConfigRules(c)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid category_overrides, got %v", errs)
	}
}

func TestValidateConfig_CategoryOverrideInvalidEffort(t *testing.T) {
	c := Config{
		Routes: map[string]string{
			"deep": "gpt-5.3-codex",
		},
		Models: map[string]ModelDef{
			"gpt-5.3-codex": {Command: "codex", Args: []string{}},
		},
		CategoryOverrides: map[string]CategoryOverride{
			"deep": {ReasoningEffort: "ultra"},
		},
	}
	errs := validateConfigRules(c)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for invalid reasoning_effort, got %d: %v", len(errs), errs)
	}
	if errs[0].Warn {
		t.Errorf("expected Warn=false (hard error) for invalid reasoning_effort, got Warn=true")
	}
}
