package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// copyArgs returns a copy of the base args slice so callers can safely append without mutating ModelDef.Args.
func copyArgs(base []string) []string {
	result := make([]string, len(base))
	copy(result, base)
	return result
}

// runGemini invokes the Gemini CLI using --output-format stream-json so that the
// init event arrives on stdout within seconds, naturally preventing firstOutputTimeout
// from firing during cold start. Session IDs are stored per-CWD and reused via
// --resume on subsequent calls.
func runGemini(ctx context.Context, opts runOptions, sessions *geminiSessionStore) (cliResult, error) {
	args := copyArgs(opts.ModelDef.Args)
	args = append(args, "-p", opts.Prompt, "--approval-mode=yolo", "-o", "stream-json")
	if sessions != nil {
		if id := sessions.get(opts.CWD); id != "" {
			args = append(args, "--resume", id)
		}
	}
	return runGeminiStream(ctx, cliRequest{
		Command:     opts.ModelDef.Command,
		Args:        args,
		CWD:         opts.CWD,
		Timeout:     opts.Timeout,
		ErrorPrefix: "Gemini CLI",
	}, sessions)
}

// geminiParseResult carries the outcome of the stdout scanner goroutine.
type geminiParseResult struct {
	text      string
	sessionID string
	err       error
}

// runGeminiStream executes the Gemini CLI and parses its stream-json output
// line-by-line. Completion is detected via the "result" event, eliminating the
// need for the stability-timeout heuristic used by runCli. The firstOutputTimeout
// still guards against the process never emitting an init event.
func runGeminiStream(ctx context.Context, req cliRequest, sessions *geminiSessionStore) (cliResult, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(req.Timeout.MaxTimeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, req.Command, req.Args...) //nolint:gosec
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

	tracker := &activityTracker{}

	if err := cmd.Start(); err != nil {
		return cliResult{}, fmt.Errorf("%s: start: %w", req.ErrorPrefix, err)
	}

	parseDone := make(chan geminiParseResult, 1)

	var readerWg sync.WaitGroup
	readerWg.Add(2)

	// stderr: alive signals only.
	go func() {
		defer readerWg.Done()
		io.Copy(io.MultiWriter(io.Discard, aliveWriterAdapter{tracker}), stderrPipe) //nolint:errcheck,gosec
	}()

	// stdout: parse JSONL events, accumulate assistant content.
	// Continues draining after sending to parseDone so cmd.Wait() is safe.
	go func() {
		defer readerWg.Done()
		var content strings.Builder
		var sessionID string
		var resultSent bool

		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			if resultSent || line == "" {
				continue // drain remaining output after result
			}
			var event geminiStreamEvent
			if jsonErr := json.Unmarshal([]byte(line), &event); jsonErr != nil {
				tracker.touch(false) // non-JSON line — treat as alive signal
				continue
			}
			switch event.Type {
			case "init":
				sessionID = event.SessionID
				tracker.touch(true) // first stdout → disarms firstOutputTimeout
			case "message":
				if event.Role == "assistant" {
					content.WriteString(event.Content)
				}
				tracker.touch(false)
			case "result":
				resultSent = true
				if event.Status != "success" {
					parseDone <- geminiParseResult{err: fmt.Errorf("gemini result status: %s", event.Status)}
				} else {
					parseDone <- geminiParseResult{text: content.String(), sessionID: sessionID}
				}
			case "error":
				resultSent = true
				parseDone <- geminiParseResult{err: fmt.Errorf("gemini stream error event")}
			default:
				tracker.touch(false)
			}
		}
		if !resultSent {
			switch {
			case scanner.Err() != nil:
				parseDone <- geminiParseResult{err: fmt.Errorf("stdout read: %w", scanner.Err())}
			case content.Len() > 0:
				// Process exited cleanly without a result event but accumulated content.
				parseDone <- geminiParseResult{text: content.String(), sessionID: sessionID}
			default:
				parseDone <- geminiParseResult{err: fmt.Errorf("gemini: process exited without result event")}
			}
		}
	}()

	waitCh := make(chan error, 1)
	go func() {
		readerWg.Wait()
		waitCh <- cmd.Wait()
	}()

	startTime := time.Now()
	stabilityDur := time.Duration(req.Timeout.StabilityTimeoutMs) * time.Millisecond
	firstOutputDur := time.Duration(req.Timeout.FirstOutputTimeoutMs) * time.Millisecond
	ticker := time.NewTicker(time.Duration(stabilityPollIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	clearSession := func() {
		if sessions != nil {
			sessions.set(req.CWD, "")
		}
	}
	handleParseResult := func(res geminiParseResult) (cliResult, error) {
		if res.err != nil {
			clearSession()
			return cliResult{}, fmt.Errorf("%s: %w", req.ErrorPrefix, res.err)
		}
		if sessions != nil && res.sessionID != "" {
			sessions.set(req.CWD, res.sessionID)
		}
		return cliResult{Text: strings.TrimSpace(res.text)}, nil
	}

	for {
		select {
		case res := <-parseDone:
			cancel()
			<-waitCh
			return handleParseResult(res)

		case waitErr := <-waitCh:
			// Process exited before parseDone — check for a buffered result.
			select {
			case res := <-parseDone:
				return handleParseResult(res)
			default:
			}
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				clearSession()
				return cliResult{}, fmt.Errorf("%w: %s timed out after %dms",
					ErrTimeout, req.ErrorPrefix, req.Timeout.MaxTimeoutMs)
			}
			if waitErr != nil {
				var exitErr *exec.ExitError
				if errors.As(waitErr, &exitErr) {
					clearSession()
					return cliResult{}, fmt.Errorf("%s exited with code %d",
						req.ErrorPrefix, exitErr.ExitCode())
				}
				clearSession()
				return cliResult{}, waitErr
			}
			clearSession()
			return cliResult{}, fmt.Errorf("%s: exited without result event", req.ErrorPrefix)

		case <-ctx.Done():
			<-waitCh
			clearSession()
			return cliResult{}, fmt.Errorf("%w: %s timed out after %dms",
				ErrTimeout, req.ErrorPrefix, req.Timeout.MaxTimeoutMs)

		case <-ticker.C:
			lastAlive, lastStdout := tracker.Snapshot()
			if lastStdout.IsZero() {
				// No init event yet — check firstOutputTimeout.
				aliveQuiet := lastAlive.IsZero() || time.Since(lastAlive) > stabilityDur
				if aliveQuiet && time.Since(startTime) > firstOutputDur {
					cancel()
					<-waitCh
					clearSession()
					return cliResult{}, fmt.Errorf("%w: %s first-output timeout after %dms",
						ErrTimeout, req.ErrorPrefix, req.Timeout.FirstOutputTimeoutMs)
				}
			}
			// After init event arrives, wait for result event — no stability check needed.
		}
	}
}

func runCli(parent context.Context, req cliRequest) (cliResult, error) {
	ctx, cancel := context.WithTimeout(parent, time.Duration(req.Timeout.MaxTimeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, req.Command, req.Args...) //nolint:gosec // command and args come from local config, not user input
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
		io.Copy(io.MultiWriter(&stdoutBuf, stdoutWriterAdapter{tracker}), stdoutPipe) //nolint:errcheck,gosec
	}()
	go func() {
		defer readerWg.Done()
		io.Copy(io.MultiWriter(&stderrBuf, aliveWriterAdapter{tracker}), stderrPipe) //nolint:errcheck,gosec
	}()

	type waitResult struct{ err error }
	waitCh := make(chan waitResult, 1)
	go func() {
		readerWg.Wait()
		waitCh <- waitResult{err: cmd.Wait()}
	}()

	startTime := time.Now()
	stabilityDur := time.Duration(req.Timeout.StabilityTimeoutMs) * time.Millisecond
	firstOutputDur := time.Duration(req.Timeout.FirstOutputTimeoutMs) * time.Millisecond
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
						lastFileMtime = currentMtime
						if fi.Size() > 0 {
							// 실제 내용이 있을 때만 stdout 신호 → stability 타이머 시작
							tracker.touch(true)
						} else {
							// 빈 파일 변경 (초기화 등) → alive 신호만
							tracker.touch(false)
						}
					}
				}
			}

			now := time.Now()
			lastAlive, lastStdout := tracker.Snapshot()

			if lastStdout.IsZero() {
				// No stdout yet — stay in first-output mode.
				// Fire first-output timeout once alive goes quiet AND enough time has passed.
				aliveQuiet := lastAlive.IsZero() || now.Sub(lastAlive) > stabilityDur
				if aliveQuiet && now.Sub(startTime) > firstOutputDur {
					cancel()
					<-waitCh
					return cliResult{}, fmt.Errorf("%w: %s first-output timeout after %dms",
						ErrTimeout, req.ErrorPrefix, req.Timeout.FirstOutputTimeoutMs)
				}
			} else if now.Sub(lastStdout) > stabilityDur {
				// Stdout has arrived — stability is measured against last stdout.
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
