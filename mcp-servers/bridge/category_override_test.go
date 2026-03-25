package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestResolveCategoryOverrides_PerCallWins(t *testing.T) {
	input := delegateInput{ReasoningEffort: "low"}
	modelDef := ModelDef{ReasoningEffort: "medium"}
	overrides := map[string]CategoryOverride{
		"deep": {ReasoningEffort: "high"},
	}
	effort, _, _ := resolveCategoryOverrides("deep", input, modelDef, overrides)
	if effort != "low" {
		t.Errorf("expected per-call 'low' to win, got %q", effort)
	}
}

func TestResolveCategoryOverrides_CategoryOverrideWins(t *testing.T) {
	input := delegateInput{}
	modelDef := ModelDef{ReasoningEffort: "medium"}
	overrides := map[string]CategoryOverride{
		"deep": {ReasoningEffort: "high"},
	}
	effort, _, _ := resolveCategoryOverrides("deep", input, modelDef, overrides)
	if effort != "high" {
		t.Errorf("expected category override 'high' to win, got %q", effort)
	}
}

func TestResolveCategoryOverrides_ModelDefFallback(t *testing.T) {
	input := delegateInput{}
	modelDef := ModelDef{ReasoningEffort: "medium"}
	overrides := map[string]CategoryOverride{}
	effort, _, _ := resolveCategoryOverrides("deep", input, modelDef, overrides)
	if effort != "medium" {
		t.Errorf("expected ModelDef fallback 'medium', got %q", effort)
	}
}

func TestResolveCategoryOverrides_PromptAppend(t *testing.T) {
	input := delegateInput{}
	modelDef := ModelDef{}
	overrides := map[string]CategoryOverride{
		"writing": {PromptAppend: "한국어로 작성하라."},
	}
	_, promptAppend, _ := resolveCategoryOverrides("writing", input, modelDef, overrides)
	if promptAppend != "한국어로 작성하라." {
		t.Errorf("expected prompt_append to be set, got %q", promptAppend)
	}
}

func TestResolveCategoryOverrides_NoOverride(t *testing.T) {
	input := delegateInput{}
	modelDef := ModelDef{ReasoningEffort: "low"}
	overrides := map[string]CategoryOverride{}
	effort, promptAppend, _ := resolveCategoryOverrides("quick", input, modelDef, overrides)
	if effort != "low" {
		t.Errorf("expected ModelDef effort 'low', got %q", effort)
	}
	if promptAppend != "" {
		t.Errorf("expected empty promptAppend, got %q", promptAppend)
	}
}

func TestResolveCategoryOverrides_DeveloperInstructions(t *testing.T) {
	cfg := map[string]CategoryOverride{
		"backend": {
			ReasoningEffort:       "high",
			DeveloperInstructions: "This is a Go project using Go 1.24.",
		},
	}
	_, _, devInstructions := resolveCategoryOverrides("backend", delegateInput{}, ModelDef{}, cfg)
	if devInstructions != "This is a Go project using Go 1.24." {
		t.Errorf("expected developer instructions, got %q", devInstructions)
	}
}

func TestResolveCategoryOverrides_DeveloperInstructions_Empty(t *testing.T) {
	cfg := map[string]CategoryOverride{
		"backend": {ReasoningEffort: "medium"},
	}
	_, _, devInstructions := resolveCategoryOverrides("backend", delegateInput{}, ModelDef{}, cfg)
	if devInstructions != "" {
		t.Errorf("expected empty developer instructions, got %q", devInstructions)
	}
}

func TestCategoryOverride_DeveloperInstructions_Field(t *testing.T) {
	co := CategoryOverride{
		ReasoningEffort:       "high",
		PromptAppend:          "append",
		DeveloperInstructions: "This is a Go project.",
	}
	if co.DeveloperInstructions != "This is a Go project." {
		t.Errorf("DeveloperInstructions not set correctly")
	}
}

func TestLogEntry_TokenFields(t *testing.T) {
	e := logEntry{
		Timestamp:    "2026-03-24T00:00:00Z",
		Status:       "success",
		InputTokens:  100,
		OutputTokens: 20,
	}
	data, _ := json.Marshal(e)
	s := string(data)
	if !strings.Contains(s, `"input_tokens":100`) {
		t.Errorf("input_tokens missing from log JSON: %s", s)
	}
	if !strings.Contains(s, `"output_tokens":20`) {
		t.Errorf("output_tokens missing from log JSON: %s", s)
	}
}
