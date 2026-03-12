#!/bin/bash
# bump-version.sh — oh-my-bridge 버전 일괄 업데이트
#
# Usage: ./bump-version.sh <new-version>
# Example: ./bump-version.sh 1.0.6
#
# Updates version in:
#   .claude-plugin/plugin.json          (version)
#   .claude-plugin/marketplace.json     (metadata.version + plugins[0].version)
#   CLAUDE.md                           (캐시 경로의 버전 문자열)
#   mcp-servers/bridge/main.go          (serverVersion)

set -euo pipefail

NEW_VERSION="${1:-}"
if [[ -z "$NEW_VERSION" ]]; then
  echo "Usage: ./bump-version.sh <new-version>"
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_JSON="${SCRIPT_DIR}/.claude-plugin/plugin.json"
MARKETPLACE_JSON="${SCRIPT_DIR}/.claude-plugin/marketplace.json"
CLAUDE_MD="${SCRIPT_DIR}/CLAUDE.md"
MAIN_GO="${SCRIPT_DIR}/mcp-servers/bridge/main.go"

# 현재 버전 감지
CURRENT_VERSION=$(jq -r '.version' "$PLUGIN_JSON")

echo "Bumping version: ${CURRENT_VERSION} → ${NEW_VERSION}"

# plugin.json
jq --arg v "$NEW_VERSION" '.version = $v' "$PLUGIN_JSON" > "${PLUGIN_JSON}.tmp" \
  && mv "${PLUGIN_JSON}.tmp" "$PLUGIN_JSON"
echo "  Updated: $PLUGIN_JSON"

# marketplace.json (2곳)
jq --arg v "$NEW_VERSION" '
  .metadata.version = $v |
  .plugins[0].version = $v
' "$MARKETPLACE_JSON" > "${MARKETPLACE_JSON}.tmp" \
  && mv "${MARKETPLACE_JSON}.tmp" "$MARKETPLACE_JSON"
echo "  Updated: $MARKETPLACE_JSON (metadata.version + plugins[0].version)"

# CLAUDE.md — 캐시 경로 버전 문자열 업데이트
if grep -q "$CURRENT_VERSION" "$CLAUDE_MD"; then
  sed -i '' "s/${CURRENT_VERSION}/${NEW_VERSION}/g" "$CLAUDE_MD"
  echo "  Updated: $CLAUDE_MD"
fi

# main.go — serverVersion 업데이트
if grep -q "serverVersion" "$MAIN_GO"; then
  sed -i '' "s/serverVersion *= *\"${CURRENT_VERSION}\"/serverVersion = \"${NEW_VERSION}\"/" "$MAIN_GO"
  echo "  Updated: $MAIN_GO (serverVersion)"
else
  echo "  Warning: serverVersion not found in $MAIN_GO — skipping"
fi

echo "  Committing and tagging v${NEW_VERSION}..."
git -C "$SCRIPT_DIR" add \
  "${PLUGIN_JSON}" \
  "${MARKETPLACE_JSON}" \
  "${CLAUDE_MD}" \
  "${MAIN_GO}"
git -C "$SCRIPT_DIR" commit -m "chore: bump version to ${NEW_VERSION}"
git -C "$SCRIPT_DIR" tag "v${NEW_VERSION}"

echo ""
echo "Done. Next steps:"
echo "  1. git push origin <branch> → PR → main 머지"
echo "  2. git push origin v${NEW_VERSION}  ← 머지 후 이걸 push하면 GitHub Actions 빌드 시작"
echo "  3. (2분 대기) Claude Code에서: /plugin update oh-my-bridge"
echo "  4. Claude Code 재시작"