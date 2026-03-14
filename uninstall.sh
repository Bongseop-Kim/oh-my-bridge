#!/usr/bin/env bash
set -euo pipefail

# 1. Remove MCP
if command -v claude >/dev/null 2>&1; then
  claude mcp remove bridge --scope user 2>/dev/null || true
fi

# 2. Remove skills + hooks
rm -rf "$HOME/.claude/skills/oh-my-bridge"
rm -f "$HOME/.claude/hooks/subagent-code-routing.sh"

# 3. Clean settings.json (hook entry)
SETTINGS="$HOME/.claude/settings.json"
HOOK_CMD="$HOME/.claude/hooks/subagent-code-routing.sh"
if [ -f "$SETTINGS" ] && command -v jq >/dev/null 2>&1; then
  tmp=$(mktemp)
  if jq --arg cmd "$HOOK_CMD" '
    if .hooks.SubagentStart then
      .hooks.SubagentStart |= map(.hooks |= map(select(.command != $cmd)))
      | .hooks.SubagentStart |= map(select(.hooks | length > 0))
      | if (.hooks.SubagentStart | length) == 0 then del(.hooks.SubagentStart) else . end
      | if (.hooks | keys | length) == 0 then del(.hooks) else . end
    else . end
  ' "$SETTINGS" > "$tmp" 2>/dev/null; then
    mv "$tmp" "$SETTINGS"
  else
    rm -f "$tmp"
  fi
fi

# 4. Optional: remove config + binary (flag-based, pipe-safe)
if [ "${1:-}" = "--all" ]; then
  rm -rf "$HOME/.config/oh-my-bridge"
  rm -f "$HOME/.local/bin/oh-my-bridge"
  echo "✔ oh-my-bridge fully removed (binary + config + skills) — restart Claude Code"
else
  echo "✔ oh-my-bridge uninstalled (skills + hooks + MCP removed) — restart Claude Code"
  echo "  Binary kept at ~/.local/bin/oh-my-bridge"
  echo "  Config kept at ~/.config/oh-my-bridge/"
  echo "  To remove everything: bash uninstall.sh --all"
fi
