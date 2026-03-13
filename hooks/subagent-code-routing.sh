#!/usr/bin/env bash
set -euo pipefail

SLIM="$HOME/.claude/skills/oh-my-bridge/code-routing-slim.md"
FULL="$HOME/.claude/skills/oh-my-bridge/SKILL.md"

TARGET=""
if [ -f "$SLIM" ]; then TARGET="$SLIM"
elif [ -f "$FULL" ]; then TARGET="$FULL"
else exit 0; fi

command -v jq &>/dev/null || exit 0

# 방어적 cat — 권한 문제 등으로 실패 시 조용히 종료
ctx="$(cat "$TARGET" 2>/dev/null)" || exit 0

jq -n --arg ctx "$ctx" '{
  hookSpecificOutput: {
    hookEventName: "SubagentStart",
    additionalContext: $ctx
  }
}'
