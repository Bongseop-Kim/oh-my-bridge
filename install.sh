#!/usr/bin/env bash
set -euo pipefail

REPO="Bongseop-Kim/oh-my-bridge"
INSTALL_DIR="$HOME/.local/bin"
BINARY="$INSTALL_DIR/oh-my-bridge"

# 1. Platform detection
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')

# 2. Latest version from GitHub API (jq preferred, grep/sed fallback)
API_RESPONSE=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")
if command -v jq >/dev/null 2>&1; then
  VERSION=$(echo "$API_RESPONSE" | jq -r '.tag_name' | sed 's/^v//')
else
  VERSION=$(echo "$API_RESPONSE" | grep '"tag_name"' | sed 's/.*"v\([^"]*\)".*/\1/')
fi

if [ -z "$VERSION" ]; then
  echo "ERROR: failed to detect latest version" >&2
  exit 1
fi

# 3. Skip if already current
if [ -x "$BINARY" ]; then
  INSTALLED=$("$BINARY" --version 2>/dev/null || echo "")
  if [ "$INSTALLED" = "$VERSION" ]; then
    echo "oh-my-bridge v${VERSION} already installed"
  else
    echo "Updating oh-my-bridge v${INSTALLED} → v${VERSION}..."
  fi
else
  echo "Installing oh-my-bridge v${VERSION}..."
fi

# 4. Download binary
mkdir -p "$INSTALL_DIR"
# GoReleaser name_template: {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}
URL="https://github.com/${REPO}/releases/download/v${VERSION}/oh-my-bridge_${VERSION}_${OS}_${ARCH}.tar.gz"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
if ! curl -fsSL "$URL" | tar -xz -C "$TMP"; then
  echo "ERROR: download failed — $URL" >&2
  exit 1
fi
mv "$TMP/oh-my-bridge" "$BINARY"
chmod +x "$BINARY"

# 5. Register MCP
if command -v claude >/dev/null 2>&1; then
  claude mcp remove bridge --scope user 2>/dev/null || true
  claude mcp add bridge --scope user -- "$BINARY"
else
  echo "WARNING: 'claude' not found in PATH. Register manually:"
  echo "  claude mcp add bridge --scope user -- $BINARY"
fi

# 6. Install skills + hooks + config
"$BINARY" install-skills

# Check external CLI availability
MISSING_CLIS=""
if ! command -v codex >/dev/null 2>&1; then
  MISSING_CLIS="${MISSING_CLIS}  - codex: npm install -g @openai/codex"$'\n'
fi
if ! command -v gemini >/dev/null 2>&1; then
  MISSING_CLIS="${MISSING_CLIS}  - gemini: npm install -g @google/gemini-cli"$'\n'
fi
if [ -n "$MISSING_CLIS" ]; then
  echo ""
  echo "WARNING: external CLI(s) not found — affected routes will fall back to Claude:"
  echo "$MISSING_CLIS"
fi

# 7. Cleanup legacy
rm -rf "$HOME/.claude/plugins/cache/oh-my-bridge" 2>/dev/null || true
for RC in "$HOME/.zshrc" "$HOME/.bashrc"; do
  if [ -f "$RC" ] && grep -q "oh-my-bridge()" "$RC" 2>/dev/null; then
    sed -i.bak '/# oh-my-bridge config CLI/,/^}/d' "$RC" && rm -f "${RC}.bak"
    echo "Removed legacy shell function from $RC"
  fi
done

# 8. PATH hint
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "Add to shell profile: export PATH=\"\$HOME/.local/bin:\$PATH\"" ;;
esac

echo ""
echo "✔ oh-my-bridge v${VERSION} installed"
echo "  1. Restart Claude Code"
echo "  2. Run: oh-my-bridge doctor"
