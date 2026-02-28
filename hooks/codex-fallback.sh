#!/bin/bash
# hooks/codex-fallback.sh
# PostToolUse hook: detects Codex failures and injects a fallback instruction
# via additionalContext so Claude can switch to a native SubAgent.
#
# Input (stdin): PostToolUse JSON from Claude Code hook system
# Output: JSON with additionalContext on error, empty on success
# Dependencies: jq

INPUT=$(cat)

# Detect error: explicit .error field or non-zero exit_code
HAS_ERROR=$(echo "$INPUT" | jq -r 'if (.tool_response | type) == "object" then (.tool_response.error // empty) else empty end' 2>/dev/null)
EXIT_CODE=$(echo "$INPUT" | jq -r '.tool_response.exit_code // empty' 2>/dev/null)

if [ -n "$HAS_ERROR" ] || { [ -n "$EXIT_CODE" ] && [ "$EXIT_CODE" != "0" ]; }; then
  printf '{"additionalContext": "⚠️ Codex 호출 실패. 동일 태스크를 codex-generator SubAgent 대신 Claude 네이티브 SubAgent로 재실행하라."}'
fi
# On success: no output — Claude Code receives nothing and continues normally
