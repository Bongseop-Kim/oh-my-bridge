#!/bin/bash
# hooks/codex-interceptor.sh
# PreToolUse hook: Edit|Write 호출을 가로채 Codex CLI로 라우팅
#
# Input  (stdin): PreToolUse JSON from Claude Code hook system
# Output (stdout): hookSpecificOutput JSON
#   Codex 성공 → permissionDecision: deny  (Codex가 파일 직접 수정 완료)
#   Codex 실패 → permissionDecision: allow (Claude 네이티브 편집으로 폴백)
# Dependencies: jq, codex

INPUT=$(cat)

TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // ""')
CWD=$(echo "$INPUT"       | jq -r '.cwd // "."')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // ""')

# codex CLI 존재 확인 — 없으면 Claude 네이티브 허용
if ! command -v codex &>/dev/null; then
  exit 0
fi

# 코드 파일 확장자 필터 — 바이너리·락파일·설정파일 등 제외
EXT="${FILE_PATH##*.}"
if ! echo "$EXT" | grep -qE '^(js|jsx|ts|tsx|py|go|rs|java|kt|swift|c|cpp|h|hpp|rb|php|sh|bash|css|scss|html|vue|svelte)$'; then
  exit 0
fi

TEMP_ERR=$(mktemp /tmp/ombridge-err-XXXXXX)

# ── Edit ──────────────────────────────────────────────────────────────────────
if [ "$TOOL_NAME" = "Edit" ]; then
  OLD_STRING=$(echo "$INPUT" | jq -r '.tool_input.old_string // ""')
  NEW_STRING=$(echo "$INPUT" | jq -r '.tool_input.new_string // ""')
  REPLACE_ALL=$(echo "$INPUT" | jq -r '.tool_input.replace_all // "false"')

  # printf 대신 cat + heredoc 방식으로 % 문자 오해석 방지
  TEMP_PROMPT=$(mktemp /tmp/ombridge-prompt-XXXXXX)
  {
    echo "In the file ${FILE_PATH}, find and replace the following text (replace_all=${REPLACE_ALL})."
    echo ""
    echo "FIND:"
    echo "$OLD_STRING"
    echo ""
    echo "REPLACE WITH:"
    echo "$NEW_STRING"
    echo ""
    echo "Make only this exact change. Do not modify anything else in the file."
  } > "$TEMP_PROMPT"

  CODEX_PROMPT=$(cat "$TEMP_PROMPT")
  rm -f "$TEMP_PROMPT"

  CODEX_EXIT=0
  (cd "$CWD" && codex -q -a full-auto --writable-roots "$CWD" "$CODEX_PROMPT") \
    2>"$TEMP_ERR" || CODEX_EXIT=$?

# ── Write ─────────────────────────────────────────────────────────────────────
elif [ "$TOOL_NAME" = "Write" ]; then
  CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // ""')

  # Claude가 이미 최종 내용을 결정했으므로 훅에서 직접 파일에 기록
  # (Codex에게 파일 경로를 읽으라는 모호한 프롬프트 방식 제거)
  CODEX_EXIT=0
  printf '%s' "$CONTENT" > "$FILE_PATH" 2>"$TEMP_ERR" || CODEX_EXIT=$?

else
  rm -f "$TEMP_ERR"
  exit 0
fi

# ── 결과 처리 ──────────────────────────────────────────────────────────────────
if [ "$CODEX_EXIT" -eq 0 ]; then
  rm -f "$TEMP_ERR"
  jq -n '{
    hookSpecificOutput: {
      hookEventName: "PreToolUse",
      permissionDecision: "deny",
      permissionDecisionReason: "Codex CLI가 파일 수정을 완료했습니다. Claude 네이티브 편집은 필요하지 않습니다."
    }
  }'
else
  ERR_MSG=$(head -3 "$TEMP_ERR" 2>/dev/null | tr '\n' ' ' || echo "")
  rm -f "$TEMP_ERR"
  jq -n \
    --arg reason "Codex 실패 (exit ${CODEX_EXIT}). Claude 네이티브 편집으로 폴백합니다." \
    --arg ctx "Codex 오류: ${ERR_MSG}" \
    '{
      hookSpecificOutput: {
        hookEventName: "PreToolUse",
        permissionDecision: "allow",
        permissionDecisionReason: $reason,
        additionalContext: $ctx
      }
    }'
fi
