package main

import (
	"bytes"
	"context"
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
	serverVersion        = "2.1.0"
	defaultGeminiTimeout = 120000
	defaultCodexTimeout  = 180000
	maxTimeoutMs         = 300000
)

type delegateInput struct {
	Prompt          string `json:"prompt" jsonschema:"Task prompt to send to the selected model."`
	Model           string `json:"model" jsonschema:"Target model. Supported prefixes include gemini-*, gpt-*, codex-*, and o*."`
	CWD             string `json:"cwd,omitempty" jsonschema:"Optional working directory, constrained to the configured workspace root."`
	TimeoutMs       int    `json:"timeoutMs,omitempty" jsonschema:"Optional timeout in milliseconds. Maximum 300000."`
	ReasoningEffort string `json:"reasoning_effort,omitempty" jsonschema:"Optional reasoning effort passed through to Codex."`
	BypassApprovals bool   `json:"bypassApprovals,omitempty" jsonschema:"If true, passes --dangerously-bypass-approvals-and-sandbox to Codex. Use only in trusted, sandboxed contexts."`
}

type delegateOutput struct {
	Response string `json:"response" jsonschema:"Model response text."`
	CWD      string `json:"cwd" jsonschema:"Resolved working directory used for the CLI invocation."`
	Model    string `json:"model" jsonschema:"Model identifier used for the CLI invocation."`
	Provider string `json:"provider" jsonschema:"Resolved CLI provider name."`
}

type cliResult struct {
	Text string
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delegate",
		Description: "Delegate a code generation task to the best available AI model.",
	}, delegateTool)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

func delegateTool(ctx context.Context, _ *mcp.CallToolRequest, input delegateInput) (*mcp.CallToolResult, delegateOutput, error) {
	if strings.TrimSpace(input.Prompt) == "" {
		return nil, delegateOutput{}, errors.New("prompt is required")
	}
	if strings.TrimSpace(input.Model) == "" {
		return nil, delegateOutput{}, errors.New("model is required")
	}
	if input.TimeoutMs < 0 || input.TimeoutMs > maxTimeoutMs {
		return nil, delegateOutput{}, fmt.Errorf("timeoutMs must be 0 or between 1 and %d", maxTimeoutMs)
	}

	workspaceRoot, err := getWorkspaceRoot()
	if err != nil {
		return nil, delegateOutput{}, err
	}

	provider, err := getProvider(input.Model)
	if err != nil {
		return nil, delegateOutput{}, err
	}

	resolvedCwd, err := resolveCwd(workspaceRoot, input.CWD)
	if err != nil {
		return nil, delegateOutput{}, err
	}

	timeoutMs := input.TimeoutMs
	if timeoutMs == 0 {
		if provider == "gemini" {
			timeoutMs = defaultGeminiTimeout
		} else {
			timeoutMs = defaultCodexTimeout
		}
	}

	var result cliResult
	switch provider {
	case "gemini":
		result, err = runGemini(ctx, runOptions{
			Prompt:    input.Prompt,
			CWD:       resolvedCwd,
			Model:     input.Model,
			TimeoutMs: timeoutMs,
		})
	case "codex":
		result, err = runCodex(ctx, runOptions{
			Prompt:          input.Prompt,
			CWD:             resolvedCwd,
			Model:           input.Model,
			ReasoningEffort: input.ReasoningEffort,
			BypassApprovals: input.BypassApprovals,
			TimeoutMs:       timeoutMs,
		})
	default:
		err = fmt.Errorf("unsupported provider %q", provider)
	}
	if err != nil {
		return nil, delegateOutput{}, err
	}

	output := delegateOutput{
		Response: result.Text,
		CWD:      resolvedCwd,
		Model:    input.Model,
		Provider: provider,
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result.Text},
		},
	}, output, nil
}

type runOptions struct {
	Prompt          string
	CWD             string
	Model           string
	ReasoningEffort string
	BypassApprovals bool
	TimeoutMs       int
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

func getProvider(model string) (string, error) {
	switch {
	case strings.HasPrefix(model, "gemini-"):
		return "gemini", nil
	case strings.HasPrefix(model, "gpt-"), strings.HasPrefix(model, "codex-"),
		len(model) >= 2 && model[0] == 'o' && model[1] >= '0' && model[1] <= '9':
		return "codex", nil
	default:
		return "", errors.New("unsupported model prefix. Use gemini-* or gpt-*/codex-*/o1/o3/o4-* models.")
	}
}

// runGemini invokes the Gemini CLI. The --yolo flag enables YOLO approval mode,
// which auto-approves tool calls (shell commands, file ops) while keeping the
// sandbox active. If the installed CLI supports it, --approval-mode=yolo is the
// equivalent explicit form.
func runGemini(ctx context.Context, opts runOptions) (cliResult, error) {
	return runCli(ctx, cliRequest{
		Command:     "gemini",
		Args:        []string{"-m", opts.Model, "-p", opts.Prompt, "--yolo"},
		CWD:         opts.CWD,
		TimeoutMs:   opts.TimeoutMs,
		ErrorPrefix: "Gemini CLI",
	})
}

func runCodex(ctx context.Context, opts runOptions) (cliResult, error) {
	f, err := os.CreateTemp("", "bridge-codex-*.txt")
	if err != nil {
		return cliResult{}, err
	}
	f.Close()
	outputFile := f.Name()
	defer os.Remove(outputFile)

	args := []string{
		"exec",
		"-m",
		opts.Model,
		"-o",
		outputFile,
	}
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
		Command:     "codex",
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
		return cliResult{}, fmt.Errorf("%s timed out after %dms", req.ErrorPrefix, req.TimeoutMs)
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
