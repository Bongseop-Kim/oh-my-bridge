---
name: oh-my-bridge:setup
description: Install oh-my-bridge routing skill to ~/.claude/skills/oh-my-bridge/
---

# oh-my-bridge Setup

Install the code-routing skill so Claude automatically decides when to delegate to Codex.

## Steps

1. **Check prerequisites**

```bash
codex --version || echo "ERROR: Codex CLI not installed. Run: npm install -g @openai/codex"
jq --version    || echo "ERROR: jq not installed. Run: brew install jq"
```

If either check fails, stop and ask the user to install the missing tool.

2. **Install skill**

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
description: Use when you are about to write code...
```

4. **Report to user**

Tell the user:
- Skill installed to `~/.claude/skills/oh-my-bridge/SKILL.md`
- **Restart Claude Code** for the skill to take effect
- After restart, Claude will automatically decide when to delegate code generation to Codex

## Undo

To remove the skill, run `/oh-my-bridge:uninstall`.
