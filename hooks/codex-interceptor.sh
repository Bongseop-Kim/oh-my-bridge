#!/bin/bash
# hooks/codex-interceptor.sh
# PreToolUse hook: Edit|Write 호출을 가로채 Codex CLI로 라우팅
#
# Input  (stdin): PreToolUse JSON from Claude Code hook system
# Output (stdout): hookSpecificOutput JSON
#   Codex 성공 → permissionDecision: deny  (Codex가 파일 직접 수정 완료)
#   Codex 실패 → permissionDecision: allow (Claude 네이티브 편집으로 폴백)
# Dependencies: jq, codex

LOG_FILE="${HOME}/.claude/logs/codex-usage.log"

_log() {
  local status="$1" op="$2" file="$3" exit_code="${4:-0}" error="${5:-}"
  mkdir -p "${HOME}/.claude/logs"
  jq -n \
    --arg ts "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
    --arg tool "codex-interceptor ($op)" \
    --arg status "$status" \
    --arg exit_code "$exit_code" \
    --arg error "$error" \
    --arg file "$file" \
    '{timestamp: $ts, tool: $tool, file: $file, status: $status, exit_code: $exit_code, error: $error}' \
    >> "$LOG_FILE"
}

INPUT=$(cat)

TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // ""')
CWD=$(echo "$INPUT"       | jq -r '.cwd // "."')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // ""')

# codex CLI 존재 확인 — 없으면 Claude 네이티브 허용
if ! command -v codex &>/dev/null; then
  exit 0
fi

# 경로 트래버설 검증 — ../ 패턴 차단
if echo "$FILE_PATH" | grep -q '\.\.'; then
  exit 0
fi

# 코드 파일 확장자 필터 — 블랙리스트 방식 (바이너리·락파일·이미지·미디어 제외)
EXT="${FILE_PATH##*.}"
BASENAME=$(basename "$FILE_PATH")
# 확장자가 없는 경우(EXT == BASENAME) 또는 바이너리/락파일이면 패스스루
if [[ "$EXT" == "$BASENAME" ]]; then
  exit 0
fi
if echo "$EXT" | grep -qiE '^(png|jpg|jpeg|gif|svg|ico|webp|bmp|tiff|mp4|mp3|wav|mov|avi|zip|tar|gz|bz2|xz|7z|pdf|doc|docx|xls|xlsx|ppt|pptx|bin|exe|dll|so|dylib|class|jar|wasm|lock|sum|snap|min\.js|min\.css|map)$'; then
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
  timeout 170 codex exec --full-auto -C "$CWD" "$CODEX_PROMPT" \
    2>"$TEMP_ERR" || CODEX_EXIT=$?

# ── Write ─────────────────────────────────────────────────────────────────────
elif [ "$TOOL_NAME" = "Write" ]; then
  CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // ""')

  TEMP_PROMPT=$(mktemp /tmp/ombridge-prompt-XXXXXX)
  {
    echo "Create or overwrite the file ${FILE_PATH} with exactly the following content."
    echo "Do not add, remove, or change any characters."
    echo ""
    echo "CONTENT:"
    echo "$CONTENT"
  } > "$TEMP_PROMPT"

  CODEX_PROMPT=$(cat "$TEMP_PROMPT")
  rm -f "$TEMP_PROMPT"

  CODEX_EXIT=0
  timeout 170 codex exec --full-auto -C "$CWD" "$CODEX_PROMPT" \
    2>"$TEMP_ERR" || CODEX_EXIT=$?

else
  rm -f "$TEMP_ERR"
  exit 0
fi

# ── 결과 처리 (Edit / Write 공통) ────────────────────────────────────────────────
if [ "$CODEX_EXIT" -eq 0 ]; then
  rm -f "$TEMP_ERR"
  jq -n '{
    hookSpecificOutput: {
      hookEventName: "PreToolUse",
      permissionDecision: "deny",
      permissionDecisionReason: "Codex CLI가 편집을 완료했습니다. Claude 네이티브 편집은 필요하지 않습니다."
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
