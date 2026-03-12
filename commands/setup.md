---
name: oh-my-bridge:setup
description: Install oh-my-bridge routing skill to ~/.claude/skills/oh-my-bridge/
---

# oh-my-bridge Setup

Install the code-routing and model-routing skills so Claude automatically decides when and to which model to delegate code generation.

## Steps

1. **Check prerequisites**

```bash
codex --version  || echo "ERROR: Codex CLI not installed. Run: npm install -g @openai/codex"
gemini --version || echo "ERROR: Gemini CLI not installed. Run: npm install -g @google/gemini-cli"
```

If either check fails, stop and ask the user to install the missing tool.

2. **Install skills**

```bash
mkdir -p ~/.claude/skills/oh-my-bridge
cp "${CLAUDE_PLUGIN_ROOT}/skills/code-routing.md" ~/.claude/skills/oh-my-bridge/SKILL.md
cp "${CLAUDE_PLUGIN_ROOT}/skills/model-routing.md" ~/.claude/skills/oh-my-bridge/model-routing.md
```

3. **Verify installation**

```bash
head -3 ~/.claude/skills/oh-my-bridge/SKILL.md
head -3 ~/.claude/skills/oh-my-bridge/model-routing.md
```

Expected output:
```
---
name: oh-my-bridge:code-routing
description: Use when you are about to write code...
---
---
name: oh-my-bridge:model-routing
description: Invoke after code-routing confirms delegation...
```

4. **Report to user**

Tell the user:
- Skills installed to `~/.claude/skills/oh-my-bridge/`
  - `SKILL.md` — code-routing: when to delegate
  - `model-routing.md` — model-routing: which model to use via `mcp__bridge__delegate`
- **Restart Claude Code** for the skills to take effect
- After restart, Claude will automatically decide when to delegate and route to the best available model (Codex or Gemini) via the unified bridge MCP tool

## Undo

To remove the skills, run `/oh-my-bridge:uninstall`.
