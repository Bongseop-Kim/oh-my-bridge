package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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
	case cmdCodex:
		result, err = runCodex(ctx, runOptions{
			Prompt:          finalPrompt,
			CWD:             resolvedCwd,
			ModelDef:        modelDef,
			ReasoningEffort: reasoningEffort,
			BypassApprovals: input.BypassApprovals,
			Timeout:         timeout,
		})
	case cmdGemini:
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

func statusTool(ctx context.Context, _ *mcp.CallToolRequest, _ statusInput) (*mcp.CallToolResult, statusOutput, error) {
	// Reload config and CLI availability on each invocation to pick up runtime changes.
	if err := reloadState(); err != nil {
		return nil, statusOutput{}, fmt.Errorf("config reload failed: %w", err)
	}
	configPath, err := getConfigPath()
	if err != nil {
		return nil, statusOutput{}, fmt.Errorf("cannot determine config path: %w", err)
	}
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
