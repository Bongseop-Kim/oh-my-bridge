package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

// newFakeCodexMCPServer creates a minimal bash MCP JSON-RPC server for testing.
// If captureFile is non-empty, each tools/call request body is written there.
// If errResponse is true, tools/call returns IsError=true.
func newFakeCodexMCPServer(t *testing.T, response, captureFile string, errResponse bool) string {
	return newFakeCodexMCPServerWithInitDelay(t, response, captureFile, errResponse, 0)
}

func newFakeCodexMCPServerWithInitDelay(t *testing.T, response, captureFile string, errResponse bool, initDelayMs int) string {
	t.Helper()

	var toolResultFmt string
	if errResponse {
		toolResultFmt = `{"jsonrpc":"2.0","id":%s,"result":{"content":[{"type":"text","text":"` + response + `"}],"isError":true}}`
	} else {
		toolResultFmt = `{"jsonrpc":"2.0","id":%s,"result":{"content":[{"type":"text","text":"` + response + `"}],"structuredContent":{"threadId":"test-thread-id","content":"` + response + `"}}}`
	}

	captureStmt := ""
	if captureFile != "" {
		captureStmt = fmt.Sprintf("        echo \"$line\" > %s\n", captureFile)
	}
	initDelayStmt := ""
	if initDelayMs > 0 {
		initDelayStmt = fmt.Sprintf("        sleep %.3f\n", float64(initDelayMs)/1000.0)
	}

	script := `#!/bin/bash
while IFS= read -r line; do
    id=$(echo "$line" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
    if echo "$line" | grep -q '"method":"initialize"'; then
` + initDelayStmt + `        printf '{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{}},"serverInfo":{"name":"fake-codex","version":"0.0.1"}}}\n' "$id"
    elif echo "$line" | grep -q '"method":"tools/call"'; then
` + captureStmt + `        printf '` + toolResultFmt + `\n' "$id"
    fi
done
`

	dir := t.TempDir()
	path := dir + "/fake-codex-mcp"
	if err := os.WriteFile(path, []byte(script), 0700); err != nil { //nolint:gosec
		t.Fatalf("write fake codex mcp server: %v", err)
	}
	return path
}

func TestCallCodexMCP_ReturnsContent(t *testing.T) {
	serverBin := newFakeCodexMCPServer(t, "hello from codex mcp", "", false)
	client := newCodexMCPClient(serverBin)

	result, err := callCodexMCP(context.Background(), client, runOptions{
		Prompt: "say hello",
		CWD:    t.TempDir(),
		ModelDef: ModelDef{
			Command: cmdCodex,
			Args:    []string{"exec", "--full-auto", "-m", "gpt-5.4"},
		},
		Timeout: timeoutConfig{MaxTimeoutMs: 30000},
	}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Text, "hello from codex mcp") {
		t.Errorf("expected 'hello from codex mcp' in result, got: %q", result.Text)
	}
}

func TestCallCodexMCP_WithDeveloperInstructions(t *testing.T) {
	captureFile := t.TempDir() + "/capture.txt"
	serverBin := newFakeCodexMCPServer(t, "ok", captureFile, false)
	client := newCodexMCPClient(serverBin)

	_, err := callCodexMCP(context.Background(), client, runOptions{
		Prompt:   "test",
		CWD:      t.TempDir(),
		ModelDef: ModelDef{Command: cmdCodex, Args: []string{"exec", "-m", "gpt-5.4"}},
		Timeout:  timeoutConfig{MaxTimeoutMs: 30000},
	}, "Use idiomatic Go patterns.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(captureFile) //nolint:gosec
	if err != nil {
		t.Fatalf("capture file not written: %v", err)
	}
	if !strings.Contains(string(data), "Use idiomatic Go patterns.") {
		t.Errorf("developer-instructions not forwarded to server, captured: %q", string(data))
	}
}

func TestCallCodexMCP_Reconnect_AfterInvalidate(t *testing.T) {
	serverBin := newFakeCodexMCPServer(t, "reconnected ok", "", false)
	client := newCodexMCPClient(serverBin)

	opts := runOptions{
		Prompt:   "test",
		CWD:      t.TempDir(),
		ModelDef: ModelDef{Command: cmdCodex, Args: []string{"exec", "-m", "gpt-5.4"}},
		Timeout:  timeoutConfig{MaxTimeoutMs: 30000},
	}

	if _, err := callCodexMCP(context.Background(), client, opts, ""); err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	client.invalidate()

	if _, err := callCodexMCP(context.Background(), client, opts, ""); err != nil {
		t.Fatalf("second call after invalidate failed: %v", err)
	}
}

func TestCallCodexMCP_MapsKnownArgs(t *testing.T) {
	captureFile := t.TempDir() + "/capture.txt"
	serverBin := newFakeCodexMCPServer(t, "ok", captureFile, false)
	client := newCodexMCPClient(serverBin)

	_, err := callCodexMCP(context.Background(), client, runOptions{
		Prompt: "test",
		CWD:    t.TempDir(),
		ModelDef: ModelDef{
			Command: cmdCodex,
			Args:    []string{"exec", "--full-auto", "--sandbox", "workspace-write", "-m", "gpt-5.4"},
		},
		Timeout: timeoutConfig{MaxTimeoutMs: 30000},
	}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(captureFile) //nolint:gosec
	if err != nil {
		t.Fatalf("capture file not written: %v", err)
	}
	captured := string(data)
	if !strings.Contains(captured, `"model":"gpt-5.4"`) {
		t.Fatalf("model not forwarded, captured: %q", captured)
	}
	if !strings.Contains(captured, `"approval-policy":"never"`) {
		t.Fatalf("approval policy not forwarded, captured: %q", captured)
	}
	if !strings.Contains(captured, `"sandbox":"workspace-write"`) {
		t.Fatalf("sandbox not forwarded, captured: %q", captured)
	}
}

func TestCallCodexMCP_RejectsUnknownArgs(t *testing.T) {
	serverBin := newFakeCodexMCPServer(t, "ok", "", false)
	client := newCodexMCPClient(serverBin)

	_, err := callCodexMCP(context.Background(), client, runOptions{
		Prompt:   "test",
		CWD:      t.TempDir(),
		ModelDef: ModelDef{Command: cmdCodex, Args: []string{"exec", "--not-supported"}},
		Timeout:  timeoutConfig{MaxTimeoutMs: 30000},
	}, "")
	if err == nil {
		t.Fatal("expected error for unsupported codex arg, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported codex arg") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCallCodexMCP_TimeoutIncludesColdStartConnect(t *testing.T) {
	serverBin := newFakeCodexMCPServerWithInitDelay(t, "ok", "", false, 250)
	client := newCodexMCPClient(serverBin)

	_, err := callCodexMCP(context.Background(), client, runOptions{
		Prompt:   "test",
		CWD:      t.TempDir(),
		ModelDef: ModelDef{Command: cmdCodex, Args: []string{"exec", "--full-auto", "-m", "gpt-5.4"}},
		Timeout:  timeoutConfig{MaxTimeoutMs: 50},
	}, "")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected ErrTimeout, got %v", err)
	}
}

func TestExtractModelFromArgs(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"exec", "--full-auto", "-m", "gpt-5.4"}, "gpt-5.4"},
		{[]string{"exec", "--model", "gpt-5.4"}, "gpt-5.4"},
		{[]string{"exec", "-m", "o3"}, "o3"},
		{[]string{"exec", "--full-auto"}, ""},
		{[]string{}, ""},
	}
	for _, c := range cases {
		got := extractModelFromArgs(c.args)
		if got != c.want {
			t.Errorf("extractModelFromArgs(%v) = %q, want %q", c.args, got, c.want)
		}
	}
}
