package main

import (
	"errors"
	"sync"
	"time"
)

const (
	serverName                  = "oh-my-bridge"
	serverVersion               = "2.4.4"
	defaultMaxTimeoutMs         = 1800000 // 30 minutes
	defaultFirstOutputTimeoutMs = 30000   // 30 seconds
	defaultStabilityTimeoutMs   = 10000   // 10 seconds
	stabilityPollIntervalMs     = 1000    // 1 second polling interval
)

// Config is loaded from ~/.config/oh-my-bridge/config.json at startup.
type Config struct {
	Routes            map[string]string           `json:"routes"`
	Models            map[string]ModelDef         `json:"models"`
	DefaultRoute      string                      `json:"default_route,omitempty"`
	CategoryOverrides map[string]CategoryOverride `json:"category_overrides,omitempty"`
}

// CategoryOverride holds per-category settings that override ModelDef defaults.
type CategoryOverride struct {
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	PromptAppend    string `json:"prompt_append,omitempty"`
}

// ModelDef describes how to invoke a specific model via CLI.
type ModelDef struct {
	Command         string   `json:"command"`
	Args            []string `json:"args"`
	ReasoningEffort string   `json:"reasoning_effort,omitempty"`
}

// timeoutConfig holds the three-concern timeout configuration.
type timeoutConfig struct {
	MaxTimeoutMs         int
	FirstOutputTimeoutMs int
	StabilityTimeoutMs   int
}

// activityTracker records the last time any bytes were written.
// It implements io.Writer and is used to detect output stability.
type activityTracker struct {
	mu           sync.Mutex
	lastActivity time.Time
}

func (a *activityTracker) Write(p []byte) (int, error) {
	if len(p) > 0 {
		a.mu.Lock()
		a.lastActivity = time.Now()
		a.mu.Unlock()
	}
	return len(p), nil
}

func (a *activityTracker) LastActivity() time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lastActivity
}

var (
	// ErrTimeout is returned by runCli when the context deadline is exceeded.
	ErrTimeout = errors.New("cli timeout")
	// ErrUnsupportedCommand indicates no runner exists for the configured command.
	ErrUnsupportedCommand = errors.New("unsupported command")
)

// CLI command name constants used for model dispatch.
const (
	cmdCodex  = "codex"
	cmdGemini = "gemini"
)

// Reason constants for delegateOutput.Reason and logEntry.Reason.
const (
	reasonRouteConfigured   = "route_configured"
	reasonCLINotInstalled   = "cli_not_installed"
	reasonCLIErrorTimeout   = "cli_error_timeout"
	reasonCLIErrorRateLimit = "cli_error_rate_limit"
	reasonCLIErrorCrash     = "cli_error_crash"
)

type delegateInput struct {
	Prompt               string `json:"prompt" jsonschema:"Task prompt to send to the selected model."`
	Category             string `json:"category" jsonschema:"Task routing key; must match a key in Config.Routes (e.g. deep, quick, writing — see config for full list). Unknown categories are accepted when Config.DefaultRoute is set — the default_route value is used as the fallback model key."`
	Model                string `json:"model,omitempty" jsonschema:"Optional model override. Bypasses config route lookup."`
	CWD                  string `json:"cwd,omitempty" jsonschema:"Optional working directory, constrained to the configured workspace root."`
	MaxTimeoutMs         int    `json:"maxTimeoutMs,omitempty" jsonschema:"Optional overall timeout ceiling in milliseconds. Default 1800000 (30 min)."`
	FirstOutputTimeoutMs int    `json:"firstOutputTimeoutMs,omitempty" jsonschema:"Optional timeout for first output in milliseconds. Default 30000 (30 s)."`
	StabilityTimeoutMs   int    `json:"stabilityTimeoutMs,omitempty" jsonschema:"Optional stability window in milliseconds. Default 10000 (10 s)."`
	ReasoningEffort      string `json:"reasoning_effort,omitempty" jsonschema:"Optional reasoning effort override. Overrides config default."`
	BypassApprovals      bool   `json:"bypassApprovals,omitempty" jsonschema:"If true, passes --dangerously-bypass-approvals-and-sandbox to Codex. Use only in trusted, sandboxed contexts."`
	DryRun               bool   `json:"dryRun,omitempty" jsonschema:"If true, returns routing decision without executing the CLI."`
}

type delegateOutput struct {
	Action        string `json:"action,omitempty" jsonschema:"'claude' when route is configured as claude or CLI not installed — handle directly."`
	Response      string `json:"response,omitempty" jsonschema:"Model response text."`
	CWD           string `json:"cwd,omitempty" jsonschema:"Resolved working directory used for the CLI invocation."`
	Model         string `json:"model,omitempty" jsonschema:"Model identifier used for the CLI invocation."`
	Category      string `json:"category,omitempty" jsonschema:"Task category used for route resolution."`
	Provider      string `json:"provider,omitempty" jsonschema:"Resolved CLI provider name."`
	LatencyMs     int64  `json:"latency_ms,omitempty" jsonschema:"CLI execution time in milliseconds."`
	TimedOut      bool   `json:"timed_out,omitempty" jsonschema:"True if the CLI invocation exceeded its timeout."`
	StabilityExit bool   `json:"stability_exit,omitempty" jsonschema:"True if the CLI was terminated by the stability timeout. Verify output files before trusting the response."`
	Reason        string `json:"reason,omitempty" jsonschema:"Reason for claude action."`
}

type logEntry struct {
	Timestamp     string `json:"timestamp"`
	Model         string `json:"model"`
	Provider      string `json:"provider"`
	Category      string `json:"category,omitempty"`
	LatencyMs     int64  `json:"latency_ms"`
	TimedOut      bool   `json:"timed_out"`
	StabilityExit bool   `json:"stability_exit,omitempty"`
	Status        string `json:"status"`
	Error         string `json:"error,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

// cliResult holds the text output from a CLI invocation.
type cliResult struct {
	Text          string
	StabilityExit bool // true when terminated by stability timeout (output may be complete)
}

type cliRequest struct {
	Command     string
	Args        []string
	CWD         string
	Timeout     timeoutConfig
	OutputFile  string
	ErrorPrefix string
}

type runOptions struct {
	Prompt          string
	CWD             string
	ModelDef        ModelDef
	ReasoningEffort string
	BypassApprovals bool
	Timeout         timeoutConfig
}

type statusInput struct{}

type statusOutput struct {
	Version           string                      `json:"version"`
	Routes            map[string]string           `json:"routes"`
	Models            map[string]ModelDef         `json:"models"`
	CLIStatus         map[string]bool             `json:"cli_status"`
	ConfigPath        string                      `json:"config_path"`
	CategoryOverrides map[string]CategoryOverride `json:"category_overrides,omitempty"`
}
