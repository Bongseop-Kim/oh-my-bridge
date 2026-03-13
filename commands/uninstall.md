---
name: oh-my-bridge:uninstall
description: Remove oh-my-bridge routing skill from ~/.claude/skills/oh-my-bridge/
---

# oh-my-bridge Uninstall

Remove the code-routing skill from Claude's skills directory.

## Steps

1. **Confirm with user**

Ask the user: "This will remove oh-my-bridge routing skill (code-routing) from `~/.claude/skills/oh-my-bridge/`. Claude will no longer automatically delegate code generation to Codex or Gemini. Proceed?"

If the user says no, stop.

2. **Remove SubagentStart hook**

```bash
SETTINGS="$HOME/.claude/settings.json"
HOOK_CMD="$HOME/.claude/hooks/subagent-code-routing.sh"

rm -f "$HOOK_CMD"

if [ -f "$SETTINGS" ]; then
  tmp="$(mktemp)"
  jq --arg cmd "$HOOK_CMD" '
    if .hooks.SubagentStart then
      .hooks.SubagentStart |= map(
        .hooks |= map(select(.command != $cmd))
      ) | .hooks.SubagentStart |= map(select(.hooks | length > 0))
      | if (.hooks.SubagentStart | length) == 0 then del(.hooks.SubagentStart) else . end
      | if (.hooks | keys | length) == 0 then del(.hooks) else . end
    else . end
  ' "$SETTINGS" > "$tmp" && mv "$tmp" "$SETTINGS"
  echo "OK: hook removed from settings.json"
fi
```

3. **Remove skill**

```bash
rm -rf ~/.claude/skills/oh-my-bridge
```

4. **Verify removal**

```bash
ls ~/.claude/skills/oh-my-bridge 2>/dev/null && echo "ERROR: skill directory still exists" || echo "OK: skill directory removed"
ls ~/.claude/hooks/subagent-code-routing.sh 2>/dev/null && echo "ERROR: hook script still exists" || echo "OK: hook script removed"
jq '.hooks.SubagentStart' "$HOME/.claude/settings.json" 2>/dev/null && echo "(see above)" || echo "OK: no settings.json or SubagentStart key absent"
```

5. **Report to user**

Tell the user:
- Skill removed from `~/.claude/skills/oh-my-bridge/`
- SubagentStart hook removed from `~/.claude/hooks/subagent-code-routing.sh` and `~/.claude/settings.json`
- **Restart Claude Code** for the change to take effect
- The plugin itself (MCP server Go binary) is still installed — only the routing skills were removed
- To reinstall, run `/oh-my-bridge:setup`
