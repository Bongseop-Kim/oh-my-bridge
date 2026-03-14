package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func getWorkspaceRoot() (string, error) {
	root := os.Getenv("OH_MY_BRIDGE_WORKSPACE_ROOT")
	if strings.TrimSpace(root) == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	return filepath.Abs(root)
}

func resolveCwd(workspaceRoot, cwd string) (string, error) {
	target := workspaceRoot
	if strings.TrimSpace(cwd) != "" {
		target = cwd
	}

	target, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}

	relative, err := filepath.Rel(workspaceRoot, target)
	if err != nil {
		return "", err
	}

	if relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("cwd must stay within workspace root: %s", workspaceRoot)
	}

	return target, nil
}

func writeLog(entry logEntry) {
	logMu.Lock()
	defer logMu.Unlock()
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "writeLog: UserHomeDir: %v\n", err)
		return
	}
	logDir := filepath.Join(home, ".claude", "logs")
	if err := os.MkdirAll(logDir, 0750); err != nil {
		fmt.Fprintf(os.Stderr, "writeLog: MkdirAll: %v\n", err)
		return
	}
	logPath := filepath.Join(logDir, "oh-my-bridge.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600) //nolint:gosec
	if err != nil {
		fmt.Fprintf(os.Stderr, "writeLog: OpenFile: %v\n", err)
		return
	}
	defer f.Close() //nolint:errcheck
	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "writeLog: json.Marshal: %v\n", err)
		return
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		fmt.Fprintf(os.Stderr, "writeLog: Write: %v\n", err)
	}
}

func toJSONOrEmpty(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}
