---
name: oh-my-bridge:setup
description: Install oh-my-bridge routing skill to ~/.claude/skills/oh-my-bridge/
---

# oh-my-bridge Setup

Install the code-routing skill so Claude automatically decides when and to which model to delegate code generation.

## Steps

1. **Download oh-my-bridge binary**

```bash
VERSION=$(jq -r '.version' "${CLAUDE_PLUGIN_ROOT}/.claude-plugin/plugin.json")
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
BINARY="${CLAUDE_PLUGIN_ROOT}/mcp-servers/bridge/oh-my-bridge"
URL="https://github.com/Bongseop-Kim/oh-my-bridge/releases/download/v${VERSION}/oh-my-bridge_${VERSION}_${OS}_${ARCH}.tar.gz"

if [ -x "$BINARY" ]; then
  echo "OK: binary already exists at $BINARY"
else
  echo "Downloading oh-my-bridge v${VERSION} for ${OS}/${ARCH}..."
  TMPDIR=$(mktemp -d)
  if curl -fsSL "$URL" | tar -xz -C "$TMPDIR"; then
    mv "$TMPDIR"/oh-my-bridge*/oh-my-bridge "$BINARY"
    chmod +x "$BINARY"
    rm -rf "$TMPDIR"
    echo "OK: downloaded successfully"
  else
    rm -rf "$TMPDIR"
    echo "ERROR: Download failed. Release v${VERSION} may not exist yet."
    echo "  URL: $URL"
    echo "  Check: https://github.com/Bongseop-Kim/oh-my-bridge/releases"
    exit 1
  fi
fi
```

2. **Register MCP server with absolute path in global config**

Write the bridge MCP server entry to `~/.claude.json` using the absolute cache path, so the binary is found regardless of which project directory Claude Code is opened in.

```bash
BINARY="${CLAUDE_PLUGIN_ROOT}/mcp-servers/bridge/oh-my-bridge"
CLAUDE_JSON="$HOME/.claude.json"

if [ -f "$CLAUDE_JSON" ]; then
  jq --arg bin "$BINARY" '.mcpServers.bridge = {"type":"stdio","command":$bin,"args":[]}' "$CLAUDE_JSON" > /tmp/claude.json && mv /tmp/claude.json "$CLAUDE_JSON"
else
  echo "{\"mcpServers\":{\"bridge\":{\"type\":\"stdio\",\"command\":\"$BINARY\",\"args\":[]}}}" > "$CLAUDE_JSON"
fi
echo "OK: ~/.claude.json updated with absolute path"
```

3. **Install skills**

```bash
mkdir -p ~/.claude/skills/oh-my-bridge
cp "${CLAUDE_PLUGIN_ROOT}/skills/code-routing.md" ~/.claude/skills/oh-my-bridge/SKILL.md
```

3.5. **Generate config**

```bash
CONFIG_DIR="$HOME/.config/oh-my-bridge"
CONFIG_FILE="$CONFIG_DIR/config.json"
mkdir -p "$CONFIG_DIR"

# Warn if existing config will be overwritten
if [ -f "$CONFIG_FILE" ]; then
  echo "⚠️  Existing config will be overwritten."
  echo "    If you have custom settings, back up first:"
  echo "    cp $CONFIG_FILE $CONFIG_FILE.bak"
fi

# Always write default config
cat > "$CONFIG_FILE" << 'CONF'
{
  "routes": {
    "visual-engineering": "gemini-3-pro",
    "ultrabrain": "gpt-5.3-codex",
    "deep": "gpt-5.3-codex",
    "artistry": "gemini-3-pro",
    "quick": "claude",
    "writing": "gemini-3-flash",
    "unspecified-high": "gpt-5.4",
    "unspecified-low": "claude"
  },
  "models": {
    "gpt-5.4":             {"command": "codex", "args": ["exec", "-m", "gpt-5.4"]},
    "gpt-5.3-codex":       {"command": "codex", "args": ["exec", "-m", "gpt-5.3-codex"]},
    "gpt-5.3-codex-spark": {"command": "codex", "args": ["exec", "-m", "gpt-5.3-codex-spark"]},
    "gemini-3-pro":        {"command": "gemini", "args": ["-m", "gemini-3-pro"]},
    "gemini-3-flash":      {"command": "gemini", "args": ["-m", "gemini-3-flash"]},
    "gemini-2.5-pro":      {"command": "gemini", "args": ["-m", "gemini-2.5-pro"]},
    "gemini-2.5-flash":    {"command": "gemini", "args": ["-m", "gemini-2.5-flash"]}
  }
}
CONF
echo "OK: config written to $CONFIG_FILE"
```

4. **Verify installation**

```bash
# Verify Go binary
BINARY="${CLAUDE_PLUGIN_ROOT}/mcp-servers/bridge/oh-my-bridge"
if [ -x "$BINARY" ]; then
  echo "OK: binary exists and is executable at $BINARY"
  ls -lh "$BINARY"
else
  echo "ERROR: binary missing or not executable at $BINARY"
fi

# Verify skill
head -3 ~/.claude/skills/oh-my-bridge/SKILL.md

# Verify config
cat ~/.config/oh-my-bridge/config.json | jq .routes
```

Expected output:
```
OK: binary exists and is executable at .../oh-my-bridge
-rwxr-xr-x  ...  oh-my-bridge
---
name: oh-my-bridge:code-routing
description: ...
{
  "visual-engineering": "gemini-3-pro",
  ...
}
```

5. **Report to user**

Tell the user:
- Skill installed to `~/.claude/skills/oh-my-bridge/SKILL.md`
- Config written to `~/.config/oh-my-bridge/config.json`
  - Routes (category → model) and model definitions are in the config
  - Edit the config to customize routing — `/oh-my-bridge:setup` resets it to defaults, so back up custom settings first
- **Restart Claude Code** for the skill to take effect
- After restart, Claude will automatically decide when to delegate and route to the best available model (Codex or Gemini) via the unified bridge MCP tool

## Undo

To remove the skills, run `/oh-my-bridge:uninstall`.
