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

2. **Install skills**

```bash
mkdir -p ~/.claude/skills/oh-my-bridge
cp "${CLAUDE_PLUGIN_ROOT}/skills/code-routing.md" ~/.claude/skills/oh-my-bridge/SKILL.md
```

3. **Verify installation**

```bash
head -3 ~/.claude/skills/oh-my-bridge/SKILL.md
```

Expected output:
```
---
name: oh-my-bridge:code-routing
description: ALWAYS invoke before any code change...
```

4. **Report to user**

Tell the user:
- Skill installed to `~/.claude/skills/oh-my-bridge/SKILL.md`
  - Model routing table (category → fallback chain → MCP params) is inlined in the skill
- **Restart Claude Code** for the skill to take effect
- After restart, Claude will automatically decide when to delegate and route to the best available model (Codex or Gemini) via the unified bridge MCP tool

## Undo

To remove the skills, run `/oh-my-bridge:uninstall`.
