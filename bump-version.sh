#!/bin/bash
# bump-version.sh — oh-my-bridge 버전 일괄 업데이트
#
# Usage: ./bump-version.sh <new-version>
# Example: ./bump-version.sh 1.0.6
#
# Updates version in:
#   .claude-plugin/plugin.json          (version)
#   .claude-plugin/marketplace.json     (metadata.version + plugins[0].version)
#   mcp-servers/bridge/types.go         (serverVersion)

set -euo pipefail

# Portable in-place sed: macOS requires 'sed -i ""', Linux requires 'sed -i'
sed_inplace() {
  if [[ "$OSTYPE" == darwin* ]]; then
    sed -i '' "$@"
  else
    sed -i "$@"
  fi
}

NEW_VERSION="${1:-}"
if [[ -z "$NEW_VERSION" ]]; then
  echo "Usage: ./bump-version.sh <new-version>"
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# main 브랜치에서만 실행 가능
CURRENT_BRANCH=$(git -C "$SCRIPT_DIR" rev-parse --abbrev-ref HEAD)
if [[ "$CURRENT_BRANCH" != "main" ]]; then
  echo "Error: bump-version must be run on main branch (current: ${CURRENT_BRANCH})" >&2
  exit 1
fi
PLUGIN_JSON="${SCRIPT_DIR}/.claude-plugin/plugin.json"
MARKETPLACE_JSON="${SCRIPT_DIR}/.claude-plugin/marketplace.json"
TYPES_GO="${SCRIPT_DIR}/mcp-servers/bridge/types.go"

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

# types.go — serverVersion 업데이트
TYPES_GO_UPDATED=false
if grep -qE "serverVersion *= *\"${CURRENT_VERSION}\"" "$TYPES_GO"; then
  sed_inplace "s/serverVersion\( *\)= *\"${CURRENT_VERSION}\"/serverVersion\1= \"${NEW_VERSION}\"/" "$TYPES_GO"
  if grep -qE "serverVersion *= *\"${NEW_VERSION}\"" "$TYPES_GO"; then
    echo "  Updated: $TYPES_GO (serverVersion)"
    TYPES_GO_UPDATED=true
  else
    echo "Error: sed ran but serverVersion = \"${NEW_VERSION}\" not found in $TYPES_GO — aborting" >&2
    exit 1
  fi
else
  echo "Error: serverVersion = \"${CURRENT_VERSION}\" not found in $TYPES_GO — aborting" >&2
  exit 1
fi

echo "  Committing and tagging v${NEW_VERSION}..."
GIT_ADD_FILES=("${PLUGIN_JSON}" "${MARKETPLACE_JSON}")
if [[ "$TYPES_GO_UPDATED" == true ]]; then
  GIT_ADD_FILES+=("${TYPES_GO}")
fi
git -C "$SCRIPT_DIR" add "${GIT_ADD_FILES[@]}"
git -C "$SCRIPT_DIR" commit -m "chore: bump version to ${NEW_VERSION}"
git -C "$SCRIPT_DIR" tag "v${NEW_VERSION}"
git -C "$SCRIPT_DIR" push origin main "v${NEW_VERSION}"

echo ""
echo "Done. Next steps:"
echo "  1. (2분 대기) GitHub Actions가 릴리스 완료될 때까지 대기"
echo "  2. curl -sSL https://raw.githubusercontent.com/Bongseop-Kim/oh-my-bridge/main/install.sh | bash"
echo "  3. Claude Code 재시작"