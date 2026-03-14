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
	"strings"
	"sync"
	"time"
)

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
	f.Close() //nolint:errcheck,gosec
	outputFile := f.Name()
	defer os.Remove(outputFile) //nolint:errcheck

	args := make([]string, len(opts.ModelDef.Args))
	copy(args, opts.ModelDef.Args)
	args = append(args, "-o", outputFile, "--skip-git-repo-check")

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

	data, readErr := os.ReadFile(outputFile) //nolint:gosec
	if readErr == nil {
		if text := strings.TrimSpace(string(data)); text != "" {
			return cliResult{Text: text, StabilityExit: result.StabilityExit}, nil
		}
	}

	log.Printf("runCodex: no output from stdout or output file %s; returning (done)", outputFile)
	return cliResult{Text: "(done)", StabilityExit: result.StabilityExit}, nil
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
		io.Copy(io.MultiWriter(&stdoutBuf, tracker), stdoutPipe) //nolint:errcheck,gosec
	}()
	go func() {
		defer readerWg.Done()
		io.Copy(io.MultiWriter(&stderrBuf, tracker), stderrPipe) //nolint:errcheck,gosec
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
						detail = exitErr.String()
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
