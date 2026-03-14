package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type setupPaths struct {
	skillDir     string
	hookPath     string
	settingsJSON string
	configPath   string
}

// resolveExecutable returns the real path of the running binary.
// It prints a warning if the path looks like a temporary go build/test artifact.
func resolveExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	if strings.Contains(exe, "go-build") || strings.Contains(exe, os.TempDir()) {
		fmt.Fprintf(os.Stderr, "WARNING: binary appears to be a temp build (%s)\n", exe)
		fmt.Fprintf(os.Stderr, "  setup should run from the installed binary at ~/.local/bin/oh-my-bridge\n")
	}
	return exe, nil
}

// installSkills writes the embedded skill files to the skills directory.
// Existing files are always overwritten so skills stay current with the binary.
func installSkills(skillDir string, skillMD, slimMD []byte) error {
	if err := os.MkdirAll(skillDir, 0750); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), skillMD, 0644); err != nil { //nolint:gosec
		return err
	}
	return os.WriteFile(filepath.Join(skillDir, "code-routing-slim.md"), slimMD, 0644) //nolint:gosec
}

// installHook installs the SubagentStart hook script and registers it in settings.json.
// Any existing entry with the same command path is removed before re-registration.
func installHook(hookPath, settingsPath string, hookSH []byte) error {
	if err := os.MkdirAll(filepath.Dir(hookPath), 0750); err != nil {
		return err
	}
	if err := os.WriteFile(hookPath, hookSH, 0755); err != nil { //nolint:gosec
		return err
	}

	var settings map[string]any

	data, err := os.ReadFile(settingsPath) //nolint:gosec
	switch {
	case err == nil:
		if jsonErr := json.Unmarshal(data, &settings); jsonErr != nil {
			return fmt.Errorf("~/.claude/settings.json exists but is not valid JSON — fix manually or delete it: %w", jsonErr)
		}
	case os.IsNotExist(err):
		settings = map[string]any{}
	default:
		return err
	}

	// Navigate to hooks.SubagentStart, removing any existing entry for hookPath.
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}

	var existing []any
	if raw, ok := hooks["SubagentStart"].([]any); ok {
		existing = raw
	}

	// Filter out groups that contain the same command.
	var cleaned []any
	for _, groupRaw := range existing {
		group, ok := groupRaw.(map[string]any)
		if !ok {
			cleaned = append(cleaned, groupRaw)
			continue
		}
		innerRaw, _ := group["hooks"].([]any)
		var filtered []any
		for _, hRaw := range innerRaw {
			h, ok := hRaw.(map[string]any)
			if !ok {
				filtered = append(filtered, hRaw)
				continue
			}
			cmd, ok := h["command"].(string)
			if !ok || cmd != hookPath {
				filtered = append(filtered, hRaw)
			}
		}
		if len(filtered) > 0 {
			group["hooks"] = filtered
			cleaned = append(cleaned, group)
		}
	}

	// Append new entry.
	newGroup := map[string]any{
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": hookPath,
				"timeout": 5,
			},
		},
	}
	cleaned = append(cleaned, newGroup)
	hooks["SubagentStart"] = cleaned
	settings["hooks"] = hooks

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0750); err != nil {
		return err
	}
	return writeAtomicJSON(settingsPath, settings, 0600)
}

// defaultConfig returns the canonical default configuration.
// This is the single source of truth for route and model defaults.
func defaultConfig() Config {
	return Config{
		Routes: map[string]string{
			"visual-engineering": "gemini-3-pro-preview",
			"ultrabrain":         "gpt-5.3-codex",
			"deep":               "gpt-5.3-codex",
			"artistry":           "gemini-3-pro-preview",
			"quick":              "claude",
			"writing":            "gemini-3-flash-preview",
			"unspecified-high":   "gpt-5.4",
			"unspecified-low":    "claude",
		},
		Models: map[string]ModelDef{
			"gpt-5.4":                {Command: "codex", Args: []string{"exec", "--full-auto", "-m", "gpt-5.4"}},
			"gpt-5.3-codex":          {Command: "codex", Args: []string{"exec", "--full-auto", "-m", "gpt-5.3-codex"}},
			"gpt-5.3-codex-spark":    {Command: "codex", Args: []string{"exec", "--full-auto", "-m", "gpt-5.3-codex-spark"}},
			"gemini-3-pro-preview":   {Command: "gemini", Args: []string{"-m", "gemini-3-pro-preview"}},
			"gemini-3-flash-preview": {Command: "gemini", Args: []string{"-m", "gemini-3-flash-preview"}},
			"gemini-2.5-pro":         {Command: "gemini", Args: []string{"-m", "gemini-2.5-pro"}},
			"gemini-2.5-flash":       {Command: "gemini", Args: []string{"-m", "gemini-2.5-flash"}},
		},
	}
}

// ensureConfig creates the config file if absent, or merges missing default keys
// into an existing config without overwriting user customizations.
func ensureConfig(configPath string) error {
	def := defaultConfig()

	data, err := os.ReadFile(configPath) //nolint:gosec
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// File does not exist — write full default config.
		if mkErr := os.MkdirAll(filepath.Dir(configPath), 0750); mkErr != nil {
			return mkErr
		}
		return writeAtomicJSON(configPath, def, 0600)
	}

	// File exists — merge only missing keys.
	var existing Config
	if jsonErr := json.Unmarshal(data, &existing); jsonErr != nil {
		return fmt.Errorf("existing config is not valid JSON: %w", jsonErr)
	}
	if existing.Routes == nil {
		existing.Routes = map[string]string{}
	}
	if existing.Models == nil {
		existing.Models = map[string]ModelDef{}
	}

	changed := false
	for k, v := range def.Routes {
		if _, ok := existing.Routes[k]; !ok {
			existing.Routes[k] = v
			changed = true
		}
	}
	for k, v := range def.Models {
		if _, ok := existing.Models[k]; !ok {
			existing.Models[k] = v
			changed = true
		}
	}

	if !changed {
		return nil
	}
	return writeAtomicJSON(configPath, existing, 0600)
}

// verifyInstall checks that all install-skills artifacts are in place.
// It prints a status line per check and returns the number of failures.
func verifyInstall(paths setupPaths, binaryPath string) int {
	printLine := func(label string, ok bool, detail string) {
		mark := "✔"
		if !ok {
			mark = "✘"
		}
		fmt.Printf("  %-22s %s\n", label, mark)
		if !ok && detail != "" {
			fmt.Printf("    %s\n", detail)
		}
	}

	failed := 0

	// 1. Binary executable.
	binaryOK := false
	if info, err := os.Stat(binaryPath); err == nil {
		binaryOK = info.Mode()&0111 != 0
	}
	if !binaryOK {
		failed++
	}
	printLine("binary", binaryOK, binaryPath)

	// 2. SKILL.md present.
	skillOK := false
	if _, err := os.Stat(filepath.Join(paths.skillDir, "SKILL.md")); err == nil {
		skillOK = true
	}
	if !skillOK {
		failed++
	}
	printLine("SKILL.md", skillOK, filepath.Join(paths.skillDir, "SKILL.md"))

	// 3. code-routing-slim.md present.
	slimOK := false
	if _, err := os.Stat(filepath.Join(paths.skillDir, "code-routing-slim.md")); err == nil {
		slimOK = true
	}
	if !slimOK {
		failed++
	}
	printLine("code-routing-slim", slimOK, filepath.Join(paths.skillDir, "code-routing-slim.md"))

	// 4. Hook script executable.
	hookOK := false
	if info, err := os.Stat(paths.hookPath); err == nil {
		hookOK = info.Mode()&0111 != 0
	}
	if !hookOK {
		failed++
	}
	printLine("hook script", hookOK, paths.hookPath)

	// 5. Config valid.
	configOK := false
	if raw, err := os.ReadFile(paths.configPath); err == nil { //nolint:gosec
		var c Config
		if json.Unmarshal(raw, &c) == nil {
			errs := validateConfigRules(c)
			hardErrors := 0
			for _, e := range errs {
				if !e.Warn {
					hardErrors++
				}
			}
			configOK = hardErrors == 0
		}
	}
	if !configOK {
		failed++
	}
	printLine("config", configOK, paths.configPath)

	return failed
}

// runInstallSkills installs skills, hook, and config without registering MCP.
// MCP registration is handled externally by install.sh via `claude mcp add`.
func runInstallSkills() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "oh-my-bridge: cannot determine home directory: %v\n", err)
		os.Exit(1)
	}

	exePath, err := resolveExecutable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "oh-my-bridge: cannot resolve executable path: %v\n", err)
		os.Exit(1)
	}

	paths := setupPaths{
		skillDir:     filepath.Join(home, ".claude", "skills", "oh-my-bridge"),
		hookPath:     filepath.Join(home, ".claude", "hooks", "subagent-code-routing.sh"),
		settingsJSON: filepath.Join(home, ".claude", "settings.json"),
		configPath:   filepath.Join(home, ".config", "oh-my-bridge", "config.json"),
	}

	fmt.Printf("oh-my-bridge install-skills v%s\n", serverVersion)
	fmt.Println("───────────────────────────────────────")

	steps := []struct {
		name string
		fn   func() error
	}{
		{"Install skills", func() error { return installSkills(paths.skillDir, embeddedSkillMD, embeddedSlimMD) }},
		{"Install hook", func() error { return installHook(paths.hookPath, paths.settingsJSON, embeddedHookSH) }},
		{"Ensure config", func() error { return ensureConfig(paths.configPath) }},
	}

	for _, s := range steps {
		fmt.Printf("  → %s...\n", s.name)
		if err := s.fn(); err != nil {
			fmt.Fprintf(os.Stderr, "✗ %s: %v\n", s.name, err)
			os.Exit(1)
		}
		fmt.Printf("  ✔ %s\n", s.name)
	}

	fmt.Println("───────────────────────────────────────")
	failures := verifyInstall(paths, exePath)
	if failures > 0 {
		fmt.Fprintf(os.Stderr, "\n✗ %d verification check(s) failed\n", failures)
		os.Exit(1)
	}
	fmt.Println("\n✔ install-skills complete — restart Claude Code")
}

// runSetup is deprecated. Use runInstallSkills instead.
func runSetup() {
	fmt.Fprintln(os.Stderr, "WARNING: 'setup' is deprecated, use 'install-skills' instead")
	runInstallSkills()
}

// writeAtomicJSON marshals v to indented JSON and writes it atomically to path.
func writeAtomicJSON(path string, v any, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name()) //nolint:errcheck
	if _, err := f.Write(append(data, '\n')); err != nil {
		f.Close() //nolint:errcheck
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(f.Name(), perm); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
