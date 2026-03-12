package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName           = "oh-my-bridge"
	serverVersion = "2.3.0"
	defaultGeminiTimeout = 120000
	defaultCodexTimeout  = 180000
	maxTimeoutMs         = 300000
)

// Config is loaded from ~/.config/oh-my-bridge/config.json at startup.
type Config struct {
	Routes map[string]string     `json:"routes"`
	Models map[string]ModelDef   `json:"models"`
}

// ModelDef describes how to invoke a specific model via CLI.
type ModelDef struct {
	Command         string   `json:"command"`
	Args            []string `json:"args"`
	ReasoningEffort string   `json:"reasoning_effort,omitempty"`
}

var (
	cfg           Config
	availableCLIs map[string]bool

	// ErrTimeout is returned by runCli when the context deadline is exceeded.
	ErrTimeout = errors.New("cli timeout")
)

type delegateInput struct {
	Prompt          string `json:"prompt" jsonschema:"Task prompt to send to the selected model."`
	Category        string `json:"category" jsonschema:"Task category (required): visual-engineering, ultrabrain, deep, artistry, quick, writing, unspecified-high, unspecified-low"`
	Model           string `json:"model,omitempty" jsonschema:"Optional model override. Bypasses config route lookup."`
	CWD             string `json:"cwd,omitempty" jsonschema:"Optional working directory, constrained to the configured workspace root."`
	TimeoutMs       int    `json:"timeoutMs,omitempty" jsonschema:"Optional timeout in milliseconds. Maximum 300000."`
	ReasoningEffort string `json:"reasoning_effort,omitempty" jsonschema:"Optional reasoning effort override. Overrides config default."`
	BypassApprovals bool   `json:"bypassApprovals,omitempty" jsonschema:"If true, passes --dangerously-bypass-approvals-and-sandbox to Codex. Use only in trusted, sandboxed contexts."`
}

type delegateOutput struct {
	Action    string `json:"action,omitempty" jsonschema:"'claude' when route is configured as claude or CLI not installed — handle directly."`
	Response  string `json:"response,omitempty" jsonschema:"Model response text."`
	CWD       string `json:"cwd,omitempty" jsonschema:"Resolved working directory used for the CLI invocation."`
	Model     string `json:"model,omitempty" jsonschema:"Model identifier used for the CLI invocation."`
	Category  string `json:"category,omitempty" jsonschema:"Task category used for route resolution."`
	Provider  string `json:"provider,omitempty" jsonschema:"Resolved CLI provider name."`
	LatencyMs int64  `json:"latency_ms,omitempty" jsonschema:"CLI execution time in milliseconds."`
	TimedOut  bool   `json:"timed_out,omitempty" jsonschema:"True if the CLI invocation exceeded its timeout."`
	Reason    string `json:"reason,omitempty" jsonschema:"Reason for claude action."`
}

type logEntry struct {
	Timestamp string `json:"timestamp"`
	Model     string `json:"model"`
	Provider  string `json:"provider"`
	Category  string `json:"category,omitempty"`
	LatencyMs int64  `json:"latency_ms"`
	TimedOut  bool   `json:"timed_out"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

// cliResult holds the text output from a CLI invocation.
type cliResult struct {
	Text string
}

func loadConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	configPath := filepath.Join(home, ".config", "oh-my-bridge", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("config not found at %s — run /oh-my-bridge:setup to create it", configPath)
		}
		return fmt.Errorf("reading config: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}
	return nil
}

func detectCLIs() {
	seen := make(map[string]bool)
	availableCLIs = make(map[string]bool)
	for _, def := range cfg.Models {
		if seen[def.Command] {
			continue
		}
		seen[def.Command] = true
		_, err := exec.LookPath(def.Command)
		availableCLIs[def.Command] = (err == nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "oh-my-bridge: CLI not found: %q — routes using it will be skipped\n", def.Command)
		}
	}
}

func main() {
	if err := loadConfig(); err != nil {
		log.Fatalf("oh-my-bridge: %v", err)
	}
	detectCLIs()

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

func delegateTool(ctx context.Context, _ *mcp.CallToolRequest, input delegateInput) (*mcp.CallToolResult, delegateOutput, error) {
	if strings.TrimSpace(input.Prompt) == "" {
		return nil, delegateOutput{}, errors.New("prompt is required")
	}
	if strings.TrimSpace(input.Category) == "" {
		return nil, delegateOutput{}, errors.New("category is required")
	}
	if input.TimeoutMs < 0 || input.TimeoutMs > maxTimeoutMs {
		return nil, delegateOutput{}, fmt.Errorf("timeoutMs must be between 0 and %d", maxTimeoutMs)
	}

	modelName, modelDef, skip, err := resolveModel(input.Category, input.Model)
	if err != nil {
		return nil, delegateOutput{}, err
	}
	if skip {
		reason := "Route configured as claude or CLI not installed. Handle directly."
		out := delegateOutput{
			Action:   "claude",
			Category: input.Category,
			Reason:   reason,
		}
		writeLog(logEntry{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			Model:     "claude",
			Provider:  "claude",
			Category:  input.Category,
			Status:    "claude",
		})
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

	timeoutMs := input.TimeoutMs
	if timeoutMs == 0 {
		timeoutMs = defaultTimeout(modelDef.Command)
	}

	// Per-call reasoning_effort overrides config default.
	reasoningEffort := input.ReasoningEffort
	if reasoningEffort == "" {
		reasoningEffort = modelDef.ReasoningEffort
	}

	start := time.Now()
	var result cliResult
	switch modelDef.Command {
	case "codex":
		result, err = runCodex(ctx, runOptions{
			Prompt:          input.Prompt,
			CWD:             resolvedCwd,
			ModelDef:        modelDef,
			ReasoningEffort: reasoningEffort,
			BypassApprovals: input.BypassApprovals,
			TimeoutMs:       timeoutMs,
		})
	case "gemini":
		result, err = runGemini(ctx, runOptions{
			Prompt:    input.Prompt,
			CWD:       resolvedCwd,
			ModelDef:  modelDef,
			TimeoutMs: timeoutMs,
		})
	default:
		err = fmt.Errorf("unsupported command %q for model %q", modelDef.Command, modelName)
	}
	if err != nil {
		timedOut := errors.Is(err, ErrTimeout)
		writeLog(logEntry{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			Model:     modelName,
			Provider:  modelDef.Command,
			Category:  input.Category,
			LatencyMs: time.Since(start).Milliseconds(),
			TimedOut:  timedOut,
			Status:    "error",
			Error:     err.Error(),
		})
		return nil, delegateOutput{}, err
	}

	output := delegateOutput{
		Response:  result.Text,
		CWD:       resolvedCwd,
		Model:     modelName,
		Category:  input.Category,
		Provider:  modelDef.Command,
		LatencyMs: time.Since(start).Milliseconds(),
		TimedOut:  false,
	}
	writeLog(logEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Model:     modelName,
		Provider:  modelDef.Command,
		Category:  input.Category,
		LatencyMs: output.LatencyMs,
		Status:    "success",
	})

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result.Text},
		},
	}, output, nil
}

type statusInput struct{}

type statusOutput struct {
	Routes     map[string]string     `json:"routes"`
	Models     map[string]ModelDef   `json:"models"`
	CLIStatus  map[string]bool       `json:"cli_status"`
	ConfigPath string                `json:"config_path"`
}

func statusTool(ctx context.Context, _ *mcp.CallToolRequest, _ statusInput) (*mcp.CallToolResult, statusOutput, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, statusOutput{}, fmt.Errorf("cannot determine home directory: %w", err)
	}
	configPath := filepath.Join(home, ".config", "oh-my-bridge", "config.json")
	out := statusOutput{
		Routes:     cfg.Routes,
		Models:     cfg.Models,
		CLIStatus:  availableCLIs,
		ConfigPath: configPath,
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: toJSONOrEmpty(out)},
		},
	}, out, nil
}

// resolveModel returns the model name, its definition, and whether Claude should handle directly.
// If modelOverride is set, it bypasses config route lookup.
func resolveModel(category, modelOverride string) (modelName string, def ModelDef, skip bool, err error) {
	if modelOverride != "" {
		d, ok := cfg.Models[modelOverride]
		if !ok {
			return "", ModelDef{}, false, fmt.Errorf("model override %q not found in config", modelOverride)
		}
		if !availableCLIs[d.Command] {
			return modelOverride, d, true, nil
		}
		return modelOverride, d, false, nil
	}

	routeVal, ok := cfg.Routes[category]
	if !ok {
		return "", ModelDef{}, false, fmt.Errorf("category %q not found in config routes", category)
	}
	if routeVal == "claude" {
		return "claude", ModelDef{}, true, nil
	}

	d, ok := cfg.Models[routeVal]
	if !ok {
		return "", ModelDef{}, false, fmt.Errorf("model %q (from route for category %q) not found in config models", routeVal, category)
	}
	if !availableCLIs[d.Command] {
		return routeVal, d, true, nil
	}
	return routeVal, d, false, nil
}

type runOptions struct {
	Prompt          string
	CWD             string
	ModelDef        ModelDef
	ReasoningEffort string
	BypassApprovals bool
	TimeoutMs       int
}

func defaultTimeout(command string) int {
	if command == "gemini" {
		return defaultGeminiTimeout
	}
	return defaultCodexTimeout
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

// runGemini invokes the Gemini CLI. The --yolo flag enables YOLO approval mode,
// which auto-approves tool calls (shell commands, file ops) while keeping the
// sandbox active.
func runGemini(ctx context.Context, opts runOptions) (cliResult, error) {
	args := make([]string, len(opts.ModelDef.Args))
	copy(args, opts.ModelDef.Args)
	args = append(args, "-p", opts.Prompt, "--yolo")

	return runCli(ctx, cliRequest{
		Command:     opts.ModelDef.Command,
		Args:        args,
		CWD:         opts.CWD,
		TimeoutMs:   opts.TimeoutMs,
		ErrorPrefix: "Gemini CLI",
	})
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
		TimeoutMs:   opts.TimeoutMs,
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
			return cliResult{Text: text}, nil
		}
	}

	log.Printf("runCodex: no output from stdout or output file %s; returning (done)", outputFile)
	return cliResult{Text: "(done)"}, nil
}

type cliRequest struct {
	Command     string
	Args        []string
	CWD         string
	TimeoutMs   int
	ErrorPrefix string
}

func runCli(parent context.Context, req cliRequest) (cliResult, error) {
	ctx, cancel := context.WithTimeout(parent, time.Duration(req.TimeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, req.Command, req.Args...)
	cmd.Dir = req.CWD
	cmd.Env = os.Environ()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return cliResult{}, fmt.Errorf("%w: %s timed out after %dms", ErrTimeout, req.ErrorPrefix, req.TimeoutMs)
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			detail := strings.TrimSpace(stderr.String())
			if detail == "" {
				detail = strings.TrimSpace(stdout.String())
			}
			if detail == "" && exitErr.ProcessState != nil {
				detail = exitErr.ProcessState.String()
			}
			return cliResult{}, fmt.Errorf("%s exited with code %d: %s", req.ErrorPrefix, exitErr.ExitCode(), detail)
		}
		return cliResult{}, err
	}

	return cliResult{Text: strings.TrimSpace(stdout.String())}, nil
}

func writeLog(entry logEntry) {
	go func() {
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
	}()
}

func toJSONOrEmpty(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}
