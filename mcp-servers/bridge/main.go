package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName                  = "oh-my-bridge"
	serverVersion               = "2.4.3"
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
	cfg           Config
	availableCLIs map[string]bool
	mu            sync.Mutex // protects cfg and availableCLIs
	logMu         sync.Mutex // serializes writeLog writes

	// ErrTimeout is returned by runCli when the context deadline is exceeded.
	ErrTimeout = errors.New("cli timeout")
	// ErrUnsupportedCommand indicates no runner exists for the configured command.
	ErrUnsupportedCommand = errors.New("unsupported command")
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
	Category             string `json:"category" jsonschema:"Task category (required): visual-engineering, ultrabrain, deep, artistry, quick, writing, unspecified-high, unspecified-low"`
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

func loadConfig() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("cannot determine home directory: %w", err)
	}
	configPath := filepath.Join(home, ".config", "oh-my-bridge", "config.json")
	data, err := os.ReadFile(configPath)
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

func main() {
	// config 서브커맨드 분기 — MCP 서버 기동 전에 처리
	if len(os.Args) > 1 && os.Args[1] == "config" {
		var err error
		cfg, err = loadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "oh-my-bridge: config load error: %v\n", err)
			os.Exit(1)
		}
		availableCLIs = detectCLIs(cfg)
		runConfigCommand(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Println(serverVersion)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "doctor" {
		runDoctor()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "stats" {
		runStats()
		return
	}

	// MCP 서버 모드 (기존 동작)
	var err error
	cfg, err = loadConfig()
	if err != nil {
		log.Fatalf("oh-my-bridge: %v", err)
	}
	availableCLIs = detectCLIs(cfg)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delegate",
		Description: "Delegate a code generation task to the best available AI model.",
	}, delegateTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "status",
		Description: "Return current config routes, model definitions, and CLI availability.",
	}, statusTool)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

func runDoctor() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "oh-my-bridge: cannot determine home directory: %v\n", err)
		os.Exit(1)
	}

	configPath := filepath.Join(home, ".config", "oh-my-bridge", "config.json")
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

	configData, err := os.ReadFile(configPath)
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

func delegateTool(ctx context.Context, _ *mcp.CallToolRequest, input delegateInput) (*mcp.CallToolResult, delegateOutput, error) {
	// Reload config and CLI availability on each invocation to pick up runtime changes.
	if err := reloadState(); err != nil {
		return nil, delegateOutput{}, fmt.Errorf("config reload failed: %w", err)
	}
	if strings.TrimSpace(input.Prompt) == "" {
		return nil, delegateOutput{}, errors.New("prompt is required")
	}
	if strings.TrimSpace(input.Category) == "" {
		return nil, delegateOutput{}, errors.New("category is required")
	}
	timeout, err := resolveTimeout(input)
	if err != nil {
		return nil, delegateOutput{}, err
	}

	c, clis := getState()
	modelName, modelDef, skip, skipReason, err := resolveModel(input.Category, input.Model, c, clis)
	if err != nil {
		return nil, delegateOutput{}, err
	}
	if skip {
		out := delegateOutput{
			Action:   "claude",
			Category: input.Category,
			Reason:   skipReason,
		}
		writeLog(logEntry{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			Model:     "claude",
			Provider:  "claude",
			Category:  input.Category,
			Status:    "claude",
			Reason:    skipReason,
		})
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: toJSONOrEmpty(out)},
			},
		}, out, nil
	}

	if input.DryRun {
		reason := "config route"
		if input.Model != "" {
			reason = "model override"
		}
		out := delegateOutput{
			Action:   "would_delegate",
			Model:    modelName,
			Category: input.Category,
			Provider: modelDef.Command,
			Reason:   reason,
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: toJSONOrEmpty(out)},
			},
		}, out, nil
	}

	workspaceRoot, err := getWorkspaceRoot()
	if err != nil {
		return nil, delegateOutput{}, err
	}

	resolvedCwd, err := resolveCwd(workspaceRoot, input.CWD)
	if err != nil {
		return nil, delegateOutput{}, err
	}

	reasoningEffort, promptAppend := resolveCategoryOverrides(input.Category, input, modelDef, c.CategoryOverrides)
	finalPrompt := input.Prompt
	if promptAppend != "" {
		finalPrompt = input.Prompt + "\n\n" + promptAppend
	}

	start := time.Now()
	var result cliResult
	switch modelDef.Command {
	case "codex":
		result, err = runCodex(ctx, runOptions{
			Prompt:          finalPrompt,
			CWD:             resolvedCwd,
			ModelDef:        modelDef,
			ReasoningEffort: reasoningEffort,
			BypassApprovals: input.BypassApprovals,
			Timeout:         timeout,
		})
	case "gemini":
		result, err = runGemini(ctx, runOptions{
			Prompt:   finalPrompt,
			CWD:      resolvedCwd,
			ModelDef: modelDef,
			Timeout:  timeout,
		})
	default:
		err = fmt.Errorf("%w: %q for model %q", ErrUnsupportedCommand, modelDef.Command, modelName)
	}
	if err != nil {
		if errors.Is(err, ErrUnsupportedCommand) {
			return nil, delegateOutput{}, err
		}
		timedOut := errors.Is(err, ErrTimeout)
		errMsg := err.Error()
		errReason := classifyCliError(err)
		writeLog(logEntry{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			Model:     modelName,
			Provider:  modelDef.Command,
			Category:  input.Category,
			LatencyMs: time.Since(start).Milliseconds(),
			TimedOut:  timedOut,
			Status:    "cli_error",
			Error:     errMsg,
			Reason:    errReason,
		})
		out := delegateOutput{
			Action:   "claude",
			Category: input.Category,
			Model:    modelName,
			Provider: modelDef.Command,
			TimedOut: timedOut,
			Reason:   fmt.Sprintf("%s: %s", errReason, errMsg),
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: toJSONOrEmpty(out)}},
		}, out, nil
	}

	responseText := result.Text
	if result.StabilityExit {
		responseText = "[WARNING: stability-exit — output may be incomplete, verify generated files]\n\n" + responseText
	}

	output := delegateOutput{
		Response:      responseText,
		CWD:           resolvedCwd,
		Model:         modelName,
		Category:      input.Category,
		Provider:      modelDef.Command,
		LatencyMs:     time.Since(start).Milliseconds(),
		StabilityExit: result.StabilityExit,
	}
	writeLog(logEntry{
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Model:         modelName,
		Provider:      modelDef.Command,
		Category:      input.Category,
		LatencyMs:     output.LatencyMs,
		StabilityExit: result.StabilityExit,
		Status:        "success",
	})

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: responseText},
		},
	}, output, nil
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

func statusTool(ctx context.Context, _ *mcp.CallToolRequest, _ statusInput) (*mcp.CallToolResult, statusOutput, error) {
	// Reload config and CLI availability on each invocation to pick up runtime changes.
	if err := reloadState(); err != nil {
		return nil, statusOutput{}, fmt.Errorf("config reload failed: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, statusOutput{}, fmt.Errorf("cannot determine home directory: %w", err)
	}
	configPath := filepath.Join(home, ".config", "oh-my-bridge", "config.json")
	c, clis := getState()
	out := statusOutput{
		Version:           serverVersion,
		Routes:            c.Routes,
		Models:            c.Models,
		CLIStatus:         clis,
		ConfigPath:        configPath,
		CategoryOverrides: c.CategoryOverrides,
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: toJSONOrEmpty(out)},
		},
	}, out, nil
}

// resolveModel returns the model name, its definition, and whether Claude should handle directly.
// If modelOverride is set, it bypasses config route lookup.
func resolveModel(category, modelOverride string, c Config, clis map[string]bool) (modelName string, def ModelDef, skip bool, reason string, err error) {
	if modelOverride != "" {
		d, ok := c.Models[modelOverride]
		if !ok {
			return "", ModelDef{}, false, "", fmt.Errorf("model override %q not found in config", modelOverride)
		}
		if !clis[d.Command] {
			return modelOverride, d, true, reasonCLINotInstalled, nil
		}
		return modelOverride, d, false, "", nil
	}

	routeVal, ok := c.Routes[category]
	if !ok {
		if c.DefaultRoute != "" {
			routeVal = c.DefaultRoute
		} else {
			return "", ModelDef{}, false, "", fmt.Errorf("category %q not found in config routes", category)
		}
	}
	if routeVal == "claude" {
		return "claude", ModelDef{}, true, reasonRouteConfigured, nil
	}

	d, ok := c.Models[routeVal]
	if !ok {
		return "", ModelDef{}, false, "", fmt.Errorf("model %q (from route for category %q) not found in config models", routeVal, category)
	}
	if !clis[d.Command] {
		return routeVal, d, true, reasonCLINotInstalled, nil
	}
	return routeVal, d, false, "", nil
}

// resolveCategoryOverrides returns the effective reasoningEffort and promptAppend
// for a given category. Priority: per-call input > category_overrides > ModelDef default.
func resolveCategoryOverrides(category string, input delegateInput, modelDef ModelDef, overrides map[string]CategoryOverride) (reasoningEffort, promptAppend string) {
	reasoningEffort = modelDef.ReasoningEffort
	if co, ok := overrides[category]; ok {
		if co.ReasoningEffort != "" {
			reasoningEffort = co.ReasoningEffort
		}
		promptAppend = co.PromptAppend
	}
	if input.ReasoningEffort != "" {
		reasoningEffort = input.ReasoningEffort
	}
	return
}

func classifyCliError(err error) string {
	if errors.Is(err, ErrTimeout) {
		return reasonCLIErrorTimeout
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "429") ||
		strings.Contains(lower, "too many requests") {
		return reasonCLIErrorRateLimit
	}
	return reasonCLIErrorCrash
}

type runOptions struct {
	Prompt          string
	CWD             string
	ModelDef        ModelDef
	ReasoningEffort string
	BypassApprovals bool
	Timeout         timeoutConfig
}

// resolveTimeout builds a timeoutConfig from delegateInput, applying defaults
// for zero values and validating constraints.
func resolveTimeout(input delegateInput) (timeoutConfig, error) {
	if input.MaxTimeoutMs < 0 || input.FirstOutputTimeoutMs < 0 || input.StabilityTimeoutMs < 0 {
		return timeoutConfig{}, errors.New("timeout values must be non-negative")
	}
	cfg := timeoutConfig{
		MaxTimeoutMs:         defaultMaxTimeoutMs,
		FirstOutputTimeoutMs: defaultFirstOutputTimeoutMs,
		StabilityTimeoutMs:   defaultStabilityTimeoutMs,
	}
	if input.MaxTimeoutMs != 0 {
		cfg.MaxTimeoutMs = input.MaxTimeoutMs
	}
	if input.FirstOutputTimeoutMs != 0 {
		cfg.FirstOutputTimeoutMs = input.FirstOutputTimeoutMs
	}
	if input.StabilityTimeoutMs != 0 {
		cfg.StabilityTimeoutMs = input.StabilityTimeoutMs
	}
	if cfg.FirstOutputTimeoutMs > cfg.MaxTimeoutMs {
		return timeoutConfig{}, errors.New("firstOutputTimeoutMs must not exceed maxTimeoutMs")
	}
	if cfg.StabilityTimeoutMs > cfg.MaxTimeoutMs {
		cfg.StabilityTimeoutMs = cfg.MaxTimeoutMs
	}
	return cfg, nil
}

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

// parseGeminiJSON extracts the "response" field from Gemini --output-format json output.
// Falls back to raw text if parsing fails or the field is empty.
func parseGeminiJSON(raw string) string {
	var resp struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return raw
	}
	if resp.Response != "" {
		return resp.Response
	}
	return raw
}

// runGemini invokes the Gemini CLI with --approval-mode=yolo (auto-approves tool calls)
// and --output-format json (structured output).
func runGemini(ctx context.Context, opts runOptions) (cliResult, error) {
	args := make([]string, len(opts.ModelDef.Args))
	copy(args, opts.ModelDef.Args)
	args = append(args, "-p", opts.Prompt, "--approval-mode=yolo", "--output-format", "json")

	result, err := runCli(ctx, cliRequest{
		Command:     opts.ModelDef.Command,
		Args:        args,
		CWD:         opts.CWD,
		Timeout:     opts.Timeout,
		ErrorPrefix: "Gemini CLI",
	})
	if err != nil {
		return cliResult{}, err
	}
	result.Text = parseGeminiJSON(result.Text)
	return result, nil
}

func runCodex(ctx context.Context, opts runOptions) (cliResult, error) {
	// CreateTemp secures a unique path; we close immediately so Codex can write
	// to it via -o, then defer removal for cleanup.
	f, err := os.CreateTemp("", "bridge-codex-*.txt")
	if err != nil {
		return cliResult{}, err
	}
	f.Close()
	outputFile := f.Name()
	defer os.Remove(outputFile)

	args := make([]string, len(opts.ModelDef.Args))
	copy(args, opts.ModelDef.Args)
	args = append(args, "-o", outputFile)
	args = append(args, "--skip-git-repo-check")

	if opts.BypassApprovals {
		args = append(args, "--dangerously-bypass-approvals-and-sandbox")
	}
	if strings.TrimSpace(opts.ReasoningEffort) != "" {
		args = append(args, "--config", "model_reasoning_effort="+opts.ReasoningEffort)
	}
	if strings.TrimSpace(opts.CWD) != "" {
		args = append(args, "-C", opts.CWD)
	}
	args = append(args, opts.Prompt)

	result, err := runCli(ctx, cliRequest{
		Command:     opts.ModelDef.Command,
		Args:        args,
		CWD:         opts.CWD,
		Timeout:     opts.Timeout,
		OutputFile:  outputFile,
		ErrorPrefix: "Codex CLI",
	})
	if err != nil {
		return cliResult{}, err
	}
	if result.Text != "" {
		return result, nil
	}

	data, readErr := os.ReadFile(outputFile)
	if readErr == nil {
		if text := strings.TrimSpace(string(data)); text != "" {
			return cliResult{Text: text, StabilityExit: result.StabilityExit}, nil
		}
	}

	log.Printf("runCodex: no output from stdout or output file %s; returning (done)", outputFile)
	return cliResult{Text: "(done)", StabilityExit: result.StabilityExit}, nil
}

type cliRequest struct {
	Command     string
	Args        []string
	CWD         string
	Timeout     timeoutConfig
	OutputFile  string
	ErrorPrefix string
}

func runCli(parent context.Context, req cliRequest) (cliResult, error) {
	ctx, cancel := context.WithTimeout(parent, time.Duration(req.Timeout.MaxTimeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, req.Command, req.Args...)
	cmd.Dir = req.CWD
	cmd.Env = os.Environ()
	setupProc(cmd)
	cmd.WaitDelay = 2 * time.Second

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return cliResult{}, fmt.Errorf("%s: stdout pipe: %w", req.ErrorPrefix, err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return cliResult{}, fmt.Errorf("%s: stderr pipe: %w", req.ErrorPrefix, err)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	tracker := &activityTracker{}

	if err := cmd.Start(); err != nil {
		return cliResult{}, fmt.Errorf("%s: start: %w", req.ErrorPrefix, err)
	}

	var readerWg sync.WaitGroup
	readerWg.Add(2)
	go func() {
		defer readerWg.Done()
		io.Copy(io.MultiWriter(&stdoutBuf, tracker), stdoutPipe) //nolint:errcheck
	}()
	go func() {
		defer readerWg.Done()
		io.Copy(io.MultiWriter(&stderrBuf, tracker), stderrPipe) //nolint:errcheck
	}()

	type waitResult struct{ err error }
	waitCh := make(chan waitResult, 1)
	go func() {
		readerWg.Wait()
		waitCh <- waitResult{err: cmd.Wait()}
	}()

	startTime := time.Now()
	var lastFileMtime time.Time
	if req.OutputFile != "" {
		if fi, statErr := os.Stat(req.OutputFile); statErr == nil {
			lastFileMtime = fi.ModTime()
		}
	}
	ticker := time.NewTicker(time.Duration(stabilityPollIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case res := <-waitCh:
			if res.err != nil {
				// Check if context was cancelled (max timeout)
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					return cliResult{}, fmt.Errorf("%w: %s timed out after %dms",
						ErrTimeout, req.ErrorPrefix, req.Timeout.MaxTimeoutMs)
				}
				var exitErr *exec.ExitError
				if errors.As(res.err, &exitErr) {
					detail := strings.TrimSpace(stderrBuf.String())
					if detail == "" {
						detail = strings.TrimSpace(stdoutBuf.String())
					}
					if detail == "" && exitErr.ProcessState != nil {
						detail = exitErr.ProcessState.String()
					}
					return cliResult{}, fmt.Errorf("%s exited with code %d: %s",
						req.ErrorPrefix, exitErr.ExitCode(), detail)
				}
				return cliResult{}, res.err
			}
			return cliResult{Text: strings.TrimSpace(stdoutBuf.String())}, nil

		case <-ctx.Done():
			<-waitCh
			return cliResult{}, fmt.Errorf("%w: %s timed out after %dms",
				ErrTimeout, req.ErrorPrefix, req.Timeout.MaxTimeoutMs)

		case <-ticker.C:
			if req.OutputFile != "" {
				if fi, statErr := os.Stat(req.OutputFile); statErr == nil {
					currentMtime := fi.ModTime()
					if currentMtime.After(lastFileMtime) {
						_, _ = tracker.Write([]byte{1})
						lastFileMtime = currentMtime
					}
				}
			}

			now := time.Now()
			lastActivity := tracker.LastActivity()

			if lastActivity.IsZero() {
				if now.Sub(startTime) > time.Duration(req.Timeout.FirstOutputTimeoutMs)*time.Millisecond {
					cancel()
					<-waitCh
					return cliResult{}, fmt.Errorf("%w: %s first-output timeout after %dms",
						ErrTimeout, req.ErrorPrefix, req.Timeout.FirstOutputTimeoutMs)
				}
			} else {
				if now.Sub(lastActivity) > time.Duration(req.Timeout.StabilityTimeoutMs)*time.Millisecond {
					cancel()
					<-waitCh
					return cliResult{
						Text:          strings.TrimSpace(stdoutBuf.String()),
						StabilityExit: true,
					}, nil
				}
			}
		}
	}
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
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "writeLog: MkdirAll: %v\n", err)
		return
	}
	logPath := filepath.Join(logDir, "oh-my-bridge.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "writeLog: OpenFile: %v\n", err)
		return
	}
	defer f.Close()
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
