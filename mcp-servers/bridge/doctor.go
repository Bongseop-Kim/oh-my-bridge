package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runDoctor() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "oh-my-bridge: cannot determine home directory: %v\n", err)
		os.Exit(1)
	}

	configPath, err := getConfigPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "oh-my-bridge: cannot determine config path: %v\n", err)
		os.Exit(1)
	}
	skillPath := filepath.Join(home, ".claude", "skills", "oh-my-bridge", "SKILL.md")
	slimPath := filepath.Join(home, ".claude", "skills", "oh-my-bridge", "code-routing-slim.md")
	failed := 0

	printCheck := func(name, status string, ok bool, detail string) {
		var mark string
		switch {
		case ok:
			mark = "✔"
		case strings.Contains(strings.ToLower(status), "warn"):
			mark = "⚠"
		default:
			mark = "✘"
		}
		line := fmt.Sprintf("%-22s %-16s %s", name, status, mark)
		if detail != "" {
			line = fmt.Sprintf("%s  %s", line, detail)
		}
		fmt.Println(line)
	}

	fmt.Println("oh-my-bridge doctor")
	fmt.Println("───────────────────────────────────────")

	binaryPath, exeErr := os.Executable()
	if exeErr != nil {
		binaryPath = exeErr.Error()
		failed++
		printCheck("binary", "v"+serverVersion, false, binaryPath)
	} else {
		printCheck("binary", "v"+serverVersion, true, binaryPath)
	}

	configData, err := os.ReadFile(configPath) //nolint:gosec
	if err != nil {
		failed++
		printCheck("config", "error", false, err.Error())
	} else {
		var c Config
		if err := json.Unmarshal(configData, &c); err != nil {
			failed++
			printCheck("config", "error", false, err.Error())
		} else {
			errs := validateConfigRules(c)
			hardErrors := 0
			for _, e := range errs {
				if !e.Warn {
					hardErrors++
				}
			}
			if hardErrors > 0 {
				failed++
				printCheck("config", "invalid", false, fmt.Sprintf("(%s)", configPath))
			} else {
				printCheck("config", "ok", true, fmt.Sprintf("(%s)", configPath))
			}
		}
	}

	if _, err := os.Stat(skillPath); err != nil {
		failed++
		printCheck("skill", "not found", false, skillPath)
	} else {
		printCheck("skill", "installed", true, "")
	}

	if _, err := os.Stat(slimPath); err != nil {
		failed++
		printCheck("code-routing-slim", "not found", false, slimPath)
	} else {
		printCheck("code-routing-slim", "installed", true, "")
	}

	codexPath, err := exec.LookPath("codex")
	if err != nil {
		// Missing CLIs are warnings — routes fall back to Claude, not a fatal error.
		printCheck("codex", "not found (warn)", false, "routes using codex will fall back to Claude")
	} else {
		printCheck("codex", "found", true, fmt.Sprintf("(%s)", codexPath))
	}

	geminiPath, err := exec.LookPath("gemini")
	if err != nil {
		printCheck("gemini", "not found (warn)", false, "routes using gemini will fall back to Claude")
	} else {
		printCheck("gemini", "found", true, fmt.Sprintf("(%s)", geminiPath))
	}

	fmt.Println("───────────────────────────────────────")
	if failed == 0 {
		fmt.Println("✔ all checks passed")
		os.Exit(0)
	}

	fmt.Printf("✘ %d check(s) failed\n", failed)
	os.Exit(1)
}
