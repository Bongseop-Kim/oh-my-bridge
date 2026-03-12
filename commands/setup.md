---
name: oh-my-bridge:setup
description: Install oh-my-bridge routing skill to ~/.claude/skills/oh-my-bridge/
---

# oh-my-bridge Setup

Install the code-routing skill so Claude automatically decides when and to which model to delegate code generation.

## Steps

1. **Check & build oh-my-bridge binary**

The binary is built from source in this repository — it is not distributed via Homebrew or package managers.

```bash
BINARY="${CLAUDE_PLUGIN_ROOT}/mcp-servers/bridge/oh-my-bridge"
if [ -x "$BINARY" ]; then
  echo "OK: binary exists at $BINARY"
elif command -v go &>/dev/null; then
  echo "Building oh-my-bridge from source..."
  (cd "${CLAUDE_PLUGIN_ROOT}/mcp-servers/bridge" && go build -o oh-my-bridge .) && echo "OK: build succeeded"
else
  echo "ERROR: binary not found and Go is not installed."
  echo "Install Go from https://go.dev/dl/ then re-run setup, or pre-build manually:"
  echo "  cd ${CLAUDE_PLUGIN_ROOT}/mcp-servers/bridge && go build -o oh-my-bridge ."
  exit 1
fi
```

If the binary is missing and Go is unavailable, stop and show the install instructions.

2. **Register MCP server with absolute path in global config**

Write the bridge MCP server entry to `~/.claude/mcp.json` using the absolute cache path, so the binary is found regardless of which project directory Claude Code is opened in.

```bash
BINARY="${CLAUDE_PLUGIN_ROOT}/mcp-servers/bridge/oh-my-bridge"
MCP_JSON="$HOME/.claude/mcp.json"

if [ -f "$MCP_JSON" ]; then
  jq --arg bin "$BINARY" '.mcpServers.bridge.command = $bin' "$MCP_JSON" > /tmp/mcp.json && mv /tmp/mcp.json "$MCP_JSON"
else
  echo "{\"mcpServers\":{\"bridge\":{\"type\":\"stdio\",\"command\":\"$BINARY\",\"args\":[]}}}" > "$MCP_JSON"
fi
echo "OK: ~/.claude/mcp.json updated with absolute path"
```

3. **Install skills**

```bash
mkdir -p ~/.claude/skills/oh-my-bridge
cp "${CLAUDE_PLUGIN_ROOT}/skills/code-routing.md" ~/.claude/skills/oh-my-bridge/SKILL.md
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
```

Expected output:
```
OK: binary exists and is executable at .../oh-my-bridge
-rwxr-xr-x  ...  oh-my-bridge
---
name: oh-my-bridge:code-routing
description: ALWAYS invoke before any code change...
```

5. **Report to user**

Tell the user:
- Skill installed to `~/.claude/skills/oh-my-bridge/SKILL.md`
  - Model routing table (category → fallback chain → MCP params) is inlined in the skill
- **Restart Claude Code** for the skill to take effect
- After restart, Claude will automatically decide when to delegate and route to the best available model (Codex or Gemini) via the unified bridge MCP tool

## Undo

To remove the skills, run `/oh-my-bridge:uninstall`.
