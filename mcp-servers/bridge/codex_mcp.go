package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// codexMCPClient holds a persistent MCP session to codex mcp-server.
// The session is reconnected lazily on the next call after a failure.
type codexMCPClient struct {
	mu         sync.Mutex
	session    *mcp.ClientSession
	client     *mcp.Client
	serverCmd  string
	serverArgs []string
}

func newCodexMCPClient(serverCmd string, serverArgs ...string) *codexMCPClient {
	if len(serverArgs) == 0 {
		serverArgs = []string{"mcp-server"}
	}
	return &codexMCPClient{
		client:     mcp.NewClient(&mcp.Implementation{Name: "oh-my-bridge", Version: serverVersion}, nil),
		serverCmd:  serverCmd,
		serverArgs: serverArgs,
	}
}

func (c *codexMCPClient) getOrConnect(ctx context.Context) (*mcp.ClientSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session != nil {
		return c.session, nil
	}
	transport := &mcp.CommandTransport{
		Command: exec.Command(c.serverCmd, c.serverArgs...), //nolint:gosec
	}
	session, err := c.client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("codex mcp-server connect: %w", err)
	}
	c.session = session
	return session, nil
}

func (c *codexMCPClient) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session != nil {
		c.session.Close() //nolint:errcheck,gosec
		c.session = nil
	}
}

// extractModelFromArgs scans Args for "-m <model>" and returns the model name.
// Example: ["exec", "--full-auto", "-m", "gpt-5.4"] → "gpt-5.4"
func extractModelFromArgs(args []string) string {
	for i, arg := range args {
		if (arg == "-m" || arg == "--model") && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

type codexMCPRequestConfig struct {
	model          string
	approvalPolicy string
	sandbox        string
}

func parseCodexMCPRequestConfig(args []string, bypassApprovals bool) (codexMCPRequestConfig, error) {
	var cfg codexMCPRequestConfig

	makeSetter := func(field *string, fieldName string) func(string, string) error {
		return func(value, source string) error {
			if *field != "" && *field != value {
				return fmt.Errorf("conflicting codex %s from %s: %q conflicts with %q", fieldName, source, value, *field)
			}
			*field = value
			return nil
		}
	}
	setApprovalPolicy := makeSetter(&cfg.approvalPolicy, "approval policy")
	setSandbox := makeSetter(&cfg.sandbox, "sandbox")

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "", "exec":
			continue
		case "-m", "--model":
			if i+1 >= len(args) || strings.TrimSpace(args[i+1]) == "" {
				return codexMCPRequestConfig{}, fmt.Errorf("codex arg %q requires a model value", arg)
			}
			cfg.model = args[i+1]
			i++
		case "--full-auto":
			if err := setApprovalPolicy("never", arg); err != nil {
				return codexMCPRequestConfig{}, err
			}
			if err := setSandbox("workspace-write", arg); err != nil {
				return codexMCPRequestConfig{}, err
			}
		case "--dangerously-bypass-approvals-and-sandbox":
			if err := setApprovalPolicy("never", arg); err != nil {
				return codexMCPRequestConfig{}, err
			}
			if err := setSandbox("danger-full-access", arg); err != nil {
				return codexMCPRequestConfig{}, err
			}
		case "--ask-for-approval":
			if i+1 >= len(args) || strings.TrimSpace(args[i+1]) == "" {
				return codexMCPRequestConfig{}, fmt.Errorf("codex arg %q requires a value", arg)
			}
			if err := setApprovalPolicy(args[i+1], arg); err != nil {
				return codexMCPRequestConfig{}, err
			}
			i++
		case "--sandbox":
			if i+1 >= len(args) || strings.TrimSpace(args[i+1]) == "" {
				return codexMCPRequestConfig{}, fmt.Errorf("codex arg %q requires a value", arg)
			}
			if err := setSandbox(args[i+1], arg); err != nil {
				return codexMCPRequestConfig{}, err
			}
			i++
		default:
			return codexMCPRequestConfig{}, fmt.Errorf("unsupported codex arg %q in ModelDef.Args", arg)
		}
	}

	if bypassApprovals {
		cfg.approvalPolicy = "never"
		cfg.sandbox = "danger-full-access"
	} else {
		if cfg.approvalPolicy == "" {
			cfg.approvalPolicy = "untrusted"
		}
		if cfg.sandbox == "" {
			cfg.sandbox = "workspace-write"
		}
	}

	return cfg, nil
}

// firstTextContent returns the text of the first TextContent item in content.
func firstTextContent(content []mcp.Content) string {
	for _, c := range content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func callCodexMCP(ctx context.Context, c *codexMCPClient, opts runOptions, developerInstructions string) (cliResult, error) {
	callCtx := ctx
	var cancel context.CancelFunc = func() {}
	if opts.Timeout.MaxTimeoutMs > 0 {
		callCtx, cancel = context.WithTimeout(ctx, time.Duration(opts.Timeout.MaxTimeoutMs)*time.Millisecond)
	}
	defer cancel()

	requestCfg, err := parseCodexMCPRequestConfig(opts.ModelDef.Args, opts.BypassApprovals)
	if err != nil {
		return cliResult{}, err
	}

	session, err := c.getOrConnect(callCtx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			c.invalidate()
			return cliResult{}, fmt.Errorf("%w: codex mcp connect timed out", ErrTimeout)
		}
		return cliResult{}, err
	}

	args := map[string]any{
		"prompt": opts.Prompt,
	}
	if opts.CWD != "" {
		args["cwd"] = opts.CWD
	}
	if requestCfg.model != "" {
		args["model"] = requestCfg.model
	}
	args["approval-policy"] = requestCfg.approvalPolicy
	args["sandbox"] = requestCfg.sandbox
	if opts.ReasoningEffort != "" {
		args["config"] = map[string]any{
			"model_reasoning_effort": opts.ReasoningEffort,
		}
	}
	if developerInstructions != "" {
		args["developer-instructions"] = developerInstructions
	}

	result, err := session.CallTool(callCtx, &mcp.CallToolParams{
		Name:      "codex",
		Arguments: args,
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			c.invalidate()
			return cliResult{}, fmt.Errorf("%w: codex mcp call timed out", ErrTimeout)
		}
		if !errors.Is(err, context.Canceled) {
			c.invalidate()
		}
		return cliResult{}, fmt.Errorf("codex mcp call: %w", err)
	}
	if result.IsError {
		c.invalidate()
		if text := firstTextContent(result.Content); text != "" {
			return cliResult{}, fmt.Errorf("codex mcp error: %s", text)
		}
		return cliResult{}, fmt.Errorf("codex mcp returned error")
	}

	if text := firstTextContent(result.Content); text != "" {
		return cliResult{Text: text}, nil
	}
	return cliResult{Text: "(done)"}, nil
}
