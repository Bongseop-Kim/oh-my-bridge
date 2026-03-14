package main

import "testing"

func TestResolveCategoryOverrides_PerCallWins(t *testing.T) {
	input := delegateInput{ReasoningEffort: "low"}
	modelDef := ModelDef{ReasoningEffort: "medium"}
	overrides := map[string]CategoryOverride{
		"deep": {ReasoningEffort: "high"},
	}
	effort, _ := resolveCategoryOverrides("deep", input, modelDef, overrides)
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
	effort, _ := resolveCategoryOverrides("deep", input, modelDef, overrides)
	if effort != "high" {
		t.Errorf("expected category override 'high' to win, got %q", effort)
	}
}

func TestResolveCategoryOverrides_ModelDefFallback(t *testing.T) {
	input := delegateInput{}
	modelDef := ModelDef{ReasoningEffort: "medium"}
	overrides := map[string]CategoryOverride{}
	effort, _ := resolveCategoryOverrides("deep", input, modelDef, overrides)
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
	_, promptAppend := resolveCategoryOverrides("writing", input, modelDef, overrides)
	if promptAppend != "한국어로 작성하라." {
		t.Errorf("expected prompt_append to be set, got %q", promptAppend)
	}
}

func TestResolveCategoryOverrides_NoOverride(t *testing.T) {
	input := delegateInput{}
	modelDef := ModelDef{ReasoningEffort: "low"}
	overrides := map[string]CategoryOverride{}
	effort, promptAppend := resolveCategoryOverrides("quick", input, modelDef, overrides)
	if effort != "low" {
		t.Errorf("expected ModelDef effort 'low', got %q", effort)
	}
	if promptAppend != "" {
		t.Errorf("expected empty promptAppend, got %q", promptAppend)
	}
}
