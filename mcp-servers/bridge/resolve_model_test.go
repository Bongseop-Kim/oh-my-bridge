package main

import (
	"strings"
	"testing"
)

func TestResolveModel_ModelOverride_CLINotInstalled(t *testing.T) {
	c := Config{
		Models: map[string]ModelDef{
			"gpt": {Command: "fake-cli-xyz-notfound"},
		},
	}
	clis := map[string]bool{"fake-cli-xyz-notfound": false}
	_, _, skip, reason, err := resolveModel("deep", "gpt", c, clis)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !skip {
		t.Error("expected skip=true")
	}
	if reason != "cli_not_installed" {
		t.Errorf("expected reason=cli_not_installed, got %q", reason)
	}
}

func TestResolveModel_ModelOverride_CLIInstalled(t *testing.T) {
	c := Config{
		Models: map[string]ModelDef{
			"gpt": {Command: "sh"},
		},
	}
	clis := map[string]bool{"sh": true}
	_, _, skip, reason, err := resolveModel("deep", "gpt", c, clis)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skip {
		t.Error("expected skip=false")
	}
	if reason != "" {
		t.Errorf("expected empty reason, got %q", reason)
	}
}

func TestResolveModel_RouteConfigured_Claude(t *testing.T) {
	c := Config{
		Routes: map[string]string{"deep": "claude"},
		Models: map[string]ModelDef{},
	}
	clis := map[string]bool{}
	_, _, skip, reason, err := resolveModel("deep", "", c, clis)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !skip {
		t.Error("expected skip=true")
	}
	if reason != "route_configured" {
		t.Errorf("expected reason=route_configured, got %q", reason)
	}
}

func TestResolveModel_RouteConfigured_CLINotInstalled(t *testing.T) {
	c := Config{
		Routes: map[string]string{"deep": "gpt"},
		Models: map[string]ModelDef{
			"gpt": {Command: "fake-cli-xyz-notfound"},
		},
	}
	clis := map[string]bool{"fake-cli-xyz-notfound": false}
	_, _, skip, reason, err := resolveModel("deep", "", c, clis)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !skip {
		t.Error("expected skip=true")
	}
	if reason != "cli_not_installed" {
		t.Errorf("expected reason=cli_not_installed, got %q", reason)
	}
}

func TestResolveModel_DefaultRoute_Used(t *testing.T) {
	c := Config{
		Routes:       map[string]string{},
		Models:       map[string]ModelDef{},
		DefaultRoute: "claude",
	}
	clis := map[string]bool{}
	_, _, skip, reason, err := resolveModel("artistry", "", c, clis)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !skip {
		t.Error("expected skip=true")
	}
	if reason != "route_configured" {
		t.Errorf("expected reason=route_configured, got %q", reason)
	}
}

func TestResolveModel_DefaultRoute_NotUsed_WhenRouteExists(t *testing.T) {
	c := Config{
		Routes:       map[string]string{"artistry": "claude"},
		Models:       map[string]ModelDef{},
		DefaultRoute: "something-else",
	}
	clis := map[string]bool{}
	_, _, skip, reason, err := resolveModel("artistry", "", c, clis)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !skip {
		t.Error("expected skip=true")
	}
	if reason != "route_configured" {
		t.Errorf("expected reason=route_configured, got %q", reason)
	}
}

func TestResolveModel_NoRoute_NoDefault_Error(t *testing.T) {
	c := Config{
		Routes: map[string]string{},
		Models: map[string]ModelDef{},
	}
	clis := map[string]bool{}
	_, _, _, _, err := resolveModel("artistry", "", c, clis)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "artistry") {
		t.Errorf("expected error to mention 'artistry', got: %v", err)
	}
}
