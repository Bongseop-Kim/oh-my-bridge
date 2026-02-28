#!/bin/bash
# hooks/log-codex-usage.sh
# PostToolUse hook: logs every mcp__codex__* call to JSONL for cost/usage tracking.
#
# Input (stdin): PostToolUse JSON from Claude Code hook system
# Output: none (append-only to log file)
# Dependencies: jq, standard POSIX tools

set -euo pipefail

LOG_DIR="${HOME}/.claude/logs"
LOG_FILE="${LOG_DIR}/codex-usage.log"

# Ensure log directory exists
mkdir -p "$LOG_DIR"

# Read the full hook payload from stdin
INPUT=$(cat)

# Extract fields with jq; fall back to empty string on missing keys
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // ""' 2>/dev/null || echo "")
EXIT_CODE=$(echo "$INPUT" | jq -r '.tool_response.exit_code // ""' 2>/dev/null || echo "")
HAS_ERROR=$(echo "$INPUT" | jq -r 'if (.tool_response | type) == "object" then (.tool_response.error // "") else "" end' 2>/dev/null || echo "")
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Determine status
if [ -n "$HAS_ERROR" ] || { [ -n "$EXIT_CODE" ] && [ "$EXIT_CODE" != "0" ]; }; then
  STATUS="error"
else
  STATUS="success"
fi

# Build JSONL entry
LOG_ENTRY=$(jq -n \
  --arg ts "$TIMESTAMP" \
  --arg tool "$TOOL_NAME" \
  --arg status "$STATUS" \
  --arg exit_code "$EXIT_CODE" \
  --arg error "$HAS_ERROR" \
  '{timestamp: $ts, tool: $tool, status: $status, exit_code: $exit_code, error: $error}')

echo "$LOG_ENTRY" >> "$LOG_FILE"
