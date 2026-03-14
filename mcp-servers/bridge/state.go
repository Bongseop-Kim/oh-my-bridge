package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
)

var (
	cfg           Config
	availableCLIs map[string]bool
	mu            sync.Mutex // protects cfg and availableCLIs
	logMu         sync.Mutex // serializes writeLog writes
)

func loadConfig() (Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return Config{}, fmt.Errorf("cannot determine home directory: %w", err)
	}
	data, err := os.ReadFile(configPath) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, fmt.Errorf("config not found at %s — run /oh-my-bridge:setup to create it", configPath)
		}
		return Config{}, fmt.Errorf("reading config: %w", err)
	}
	var cfgNew Config
	if err := json.Unmarshal(data, &cfgNew); err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}
	return cfgNew, nil
}

func detectCLIs(c Config) map[string]bool {
	seen := make(map[string]bool)
	clis := make(map[string]bool)
	for _, def := range c.Models {
		if seen[def.Command] {
			continue
		}
		seen[def.Command] = true
		_, err := exec.LookPath(def.Command)
		clis[def.Command] = (err == nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "oh-my-bridge: CLI not found: %q — routes using it will be skipped\n", def.Command)
		}
	}
	return clis
}

// reloadState reloads config and detects CLI availability under a mutex lock.
// Called on each delegate/status invocation to pick up runtime config changes.
func reloadState() error {
	cfgNew, err := loadConfig()
	if err != nil {
		mu.Lock()
		hasState := cfg.Routes != nil && len(cfg.Models) > 0
		mu.Unlock()
		if hasState {
			fmt.Fprintf(os.Stderr, "oh-my-bridge: config reload failed, using stale config: %v\n", err)
			return nil
		}
		return err
	}
	clisNew := detectCLIs(cfgNew)

	mu.Lock()
	cfg = cfgNew
	availableCLIs = clisNew
	mu.Unlock()
	return nil
}

func getState() (Config, map[string]bool) {
	mu.Lock()
	c := cfg
	clis := availableCLIs
	mu.Unlock()
	return c, clis
}
