---
name: oh-my-bridge:uninstall
description: Remove oh-my-bridge routing skill from ~/.claude/skills/oh-my-bridge/
---

# oh-my-bridge Uninstall

Remove the code-routing skill from Claude's skills directory.

## Steps

1. **Confirm with user**

Ask the user: "This will remove oh-my-bridge routing skill from `~/.claude/skills/oh-my-bridge/`. Claude will no longer automatically delegate code generation to Codex or Gemini. Proceed?"

If the user says no, stop.

2. **Remove skill**

```bash
rm -rf ~/.claude/skills/oh-my-bridge
```

3. **Verify removal**

```bash
ls ~/.claude/skills/oh-my-bridge 2>/dev/null && echo "ERROR: directory still exists" || echo "Removed successfully"
```

4. **Report to user**

Tell the user:
- Skill removed from `~/.claude/skills/oh-my-bridge/`
- **Restart Claude Code** for the change to take effect
- The plugin itself (MCP server, SubAgent) is still installed — only the routing skill was removed
- To reinstall, run `/oh-my-bridge:setup`
