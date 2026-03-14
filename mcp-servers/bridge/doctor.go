package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	failed := 0

	printCheck := func(name, status string, ok bool, detail string) {
		mark := "✘"
		if ok {
			mark = "✔"
		}
		line := fmt.Sprintf("%-12s %-10s %s", name, status, mark)
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
	}
	printCheck("binary", "v"+serverVersion, true, binaryPath)

	configData, err := os.ReadFile(configPath) //nolint:gosec
	if err != nil {
		failed++
		printCheck("config", "error", false, err.Error())
	} else {
		var configRaw map[string]any
		if err := json.Unmarshal(configData, &configRaw); err != nil {
			failed++
			printCheck("config", "error", false, err.Error())
		} else {
			printCheck("config", "ok", true, fmt.Sprintf("(%s)", configPath))
		}
	}

	if _, err := os.Stat(skillPath); err != nil {
		failed++
		printCheck("skill", "not found", false, skillPath)
	} else {
		printCheck("skill", "installed", true, "")
	}

	codexPath, err := exec.LookPath("codex")
	if err != nil {
		failed++
		printCheck("codex", "not found", false, err.Error())
	} else {
		printCheck("codex", "found", true, fmt.Sprintf("(%s)", codexPath))
	}

	geminiPath, err := exec.LookPath("gemini")
	if err != nil {
		failed++
		printCheck("gemini", "not found", false, err.Error())
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
