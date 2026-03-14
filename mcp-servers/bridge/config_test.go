package main

import (
	"os"
	"path/filepath"
	"testing"
)

const fixtureConfig = `{
  "models": {
    "codex-test": {
      "command": "codex",
      "args": ["--full-auto", "-m", "o4-mini"]
    }
  }
}`

func writeFixtureConfig(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "oh-my-bridge")
	if err := os.MkdirAll(dir, 0755); err != nil { //nolint:gosec
		t.Fatalf("writeFixtureConfig: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(fixtureConfig), 0644); err != nil { //nolint:gosec
		t.Fatalf("writeFixtureConfig: %v", err)
	}
}

// TestConfig_CodexModelsHaveFullAuto verifies that all codex models in config
// include --full-auto in their args.
//
// Root cause of issue #10: without --full-auto, codex exec waits for approval
// prompts interactively, causing an indefinite hang.
func TestConfig_CodexModelsHaveFullAuto(t *testing.T) {
	writeFixtureConfig(t)

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	for name, model := range cfg.Models {
		if model.Command != "codex" {
			continue
		}
		hasFullAuto := false
		for _, arg := range model.Args {
			if arg == "--full-auto" {
				hasFullAuto = true
				break
			}
		}
		if !hasFullAuto {
			t.Errorf("model %q (command: codex) is missing --full-auto in args: %v", name, model.Args)
		}
	}
}

// TestConfig_CodexArgsOrder verifies that --full-auto appears before -m in codex args.
// codex exec --full-auto -m <model> is the correct invocation order.
func TestConfig_CodexArgsOrder(t *testing.T) {
	writeFixtureConfig(t)

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	for name, model := range cfg.Models {
		if model.Command != "codex" {
			continue
		}
		fullAutoIdx, modelIdx := -1, -1
		for i, arg := range model.Args {
			if arg == "--full-auto" {
				fullAutoIdx = i
			}
			if arg == "-m" {
				modelIdx = i
			}
		}
		if fullAutoIdx == -1 || modelIdx == -1 {
			continue // already caught by TestConfig_CodexModelsHaveFullAuto
		}
		if fullAutoIdx > modelIdx {
			t.Errorf("model %q: --full-auto should appear before -m, got args: %v", name, model.Args)
		}
	}
}
