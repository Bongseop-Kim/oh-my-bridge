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

INSTALLED_VERSION=""
if [ -x "$BINARY" ]; then
  INSTALLED_VERSION=$("$BINARY" --version 2>/dev/null || echo "")
fi

# If binary not at current version path, check older cached versions
PREV_VERSION=""
if [ -z "$INSTALLED_VERSION" ]; then
  CACHE_BASE=$(dirname "${CLAUDE_PLUGIN_ROOT}")
  for old_bin in "$CACHE_BASE"/*/mcp-servers/bridge/oh-my-bridge; do
    if [ -x "$old_bin" ]; then
      v=$("$old_bin" --version 2>/dev/null || echo "")
      if [ -n "$v" ] && [ "$v" != "$VERSION" ]; then
        PREV_VERSION="$v"
      fi
    fi
  done
fi

if [ "$INSTALLED_VERSION" = "$VERSION" ]; then
  echo "OK: binary already up to date (v${VERSION})"
else
  if [ -n "$INSTALLED_VERSION" ] || [ -n "$PREV_VERSION" ]; then
    FROM_VER="${INSTALLED_VERSION:-$PREV_VERSION}"
    echo "Updating oh-my-bridge v${FROM_VER} → v${VERSION}..."
  else
    echo "Installing oh-my-bridge v${VERSION} for ${OS}/${ARCH}..."
  fi
  TMPDIR=$(mktemp -d)
  if curl -fsSL "$URL" | tar -xz -C "$TMPDIR"; then
    if mv "$TMPDIR/oh-my-bridge" "$BINARY" && chmod +x "$BINARY"; then
      rm -rf "$TMPDIR"
      echo "OK: binary installed/updated successfully"
    else
      rm -rf "$TMPDIR"
      echo "ERROR: Failed to install binary."
      exit 1
    fi
  else
    rm -rf "$TMPDIR"
    echo "ERROR: Download failed."
    echo " URL: $URL"
    exit 1
  fi
fi
```

2. **Clean up old cached versions**

```bash
CACHE_BASE=$(dirname "${CLAUDE_PLUGIN_ROOT}")
REMOVED=0
for old_dir in "$CACHE_BASE"/*/; do
  old_ver=$(basename "$old_dir")
  if [ "$old_ver" != "$VERSION" ] && [ -d "$old_dir" ]; then
    rm -rf "$old_dir"
    echo "OK: removed old version v${old_ver}"
    REMOVED=$((REMOVED + 1))
  fi
done
if [ "$REMOVED" -eq 0 ]; then
  echo "OK: no old versions to clean up"
fi
```

3. **Register MCP server with absolute path in global config**

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

4. **Install skills**

```bash
mkdir -p ~/.claude/skills/oh-my-bridge
cp "${CLAUDE_PLUGIN_ROOT}/skills/code-routing.md" ~/.claude/skills/oh-my-bridge/SKILL.md
cp "${CLAUDE_PLUGIN_ROOT}/skills/code-routing-slim.md" ~/.claude/skills/oh-my-bridge/code-routing-slim.md
```

5. **Install SubagentStart hook**

Copy the hook script and register it in `~/.claude/settings.json` so every spawned subagent automatically receives the slim code-routing context.

```bash
mkdir -p ~/.claude/hooks
if ! cp "${CLAUDE_PLUGIN_ROOT}/hooks/subagent-code-routing.sh" ~/.claude/hooks/subagent-code-routing.sh; then
  echo "ERROR: failed to copy hook script" >&2
  exit 1
fi
if ! chmod +x ~/.claude/hooks/subagent-code-routing.sh; then
  echo "ERROR: failed to chmod hook script" >&2
  exit 1
fi

SETTINGS="$HOME/.claude/settings.json"
HOOK_CMD="$HOME/.claude/hooks/subagent-code-routing.sh"

# settings.json 없으면 초기화
[ -f "$SETTINGS" ] || echo '{}' > "$SETTINGS"

# upsert: 기존 동일 command 제거 후 재등록 (중복 완전 방지)
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
if jq --arg cmd "$HOOK_CMD" '
  (.hooks.SubagentStart // []) as $existing
  | ($existing | map(
      .hooks |= map(select(.command != $cmd))
    ) | map(select(.hooks | length > 0))
  ) as $cleaned
  | .hooks.SubagentStart = $cleaned + [{"hooks":[{"type":"command","command":$cmd,"timeout":5}]}]
' "$SETTINGS" > "$tmp" && mv "$tmp" "$SETTINGS"; then
  echo "OK: SubagentStart hook registered in $SETTINGS"
else
  rm -f "$tmp"
  echo "ERROR: failed to update $SETTINGS" >&2
  exit 1
fi
```

6. **Install shell alias**

```bash
SHELL_RC=""
if [ -f "$HOME/.zshrc" ]; then
  SHELL_RC="$HOME/.zshrc"
elif [ -f "$HOME/.bashrc" ]; then
  SHELL_RC="$HOME/.bashrc"
fi

if [ -n "$SHELL_RC" ]; then
  if grep -q "oh-my-bridge()" "$SHELL_RC" 2>/dev/null; then
    echo "OK: shell function already exists in $SHELL_RC"
  else
    cat >> "$SHELL_RC" << 'SHELLRC'

# oh-my-bridge config CLI
oh-my-bridge() {
  local bin
  bin=$(ls ~/.claude/plugins/cache/oh-my-bridge/oh-my-bridge/*/mcp-servers/bridge/oh-my-bridge 2>/dev/null | sort -V | tail -1)
  if [ -z "$bin" ]; then
    echo "oh-my-bridge: binary not found — run /oh-my-bridge:setup" >&2
    return 1
  fi
  "$bin" "$@"
}
SHELLRC
    echo "OK: shell function added to $SHELL_RC"
    echo "  Run: source $SHELL_RC  (or open a new terminal)"
  fi
else
  echo "SKIP: could not find .zshrc or .bashrc — add the alias manually"
fi
```

7. **Generate config**

```bash
CONFIG_DIR="$HOME/.config/oh-my-bridge"
CONFIG_FILE="$CONFIG_DIR/config.json"
mkdir -p "$CONFIG_DIR"

# Back up existing config before overwriting
if [ -f "$CONFIG_FILE" ]; then
  BACKUP="${CONFIG_FILE}.$(date +%Y%m%dT%H%M%S).bak"
  cp "$CONFIG_FILE" "$BACKUP"
  echo "⚠️  Existing config backed up to: $BACKUP"
fi

# Always write default config
cat > "$CONFIG_FILE" << 'CONF'
{
  "routes": {
    "visual-engineering": "gemini-3-pro-preview",
    "ultrabrain": "gpt-5.3-codex",
    "deep": "gpt-5.3-codex",
    "artistry": "gemini-3-pro-preview",
    "quick": "claude",
    "writing": "gemini-3-flash-preview",
    "unspecified-high": "gpt-5.4",
    "unspecified-low": "claude"
  },
  "models": {
    "gpt-5.4":             {"command": "codex", "args": ["exec", "--full-auto", "-m", "gpt-5.4"]},
    "gpt-5.3-codex":       {"command": "codex", "args": ["exec", "--full-auto", "-m", "gpt-5.3-codex"]},
    "gpt-5.3-codex-spark": {"command": "codex", "args": ["exec", "--full-auto", "-m", "gpt-5.3-codex-spark"]},
    "gemini-3-pro-preview":   {"command": "gemini", "args": ["-m", "gemini-3-pro-preview"]},
    "gemini-3-flash-preview": {"command": "gemini", "args": ["-m", "gemini-3-flash-preview"]},
    "gemini-2.5-pro":         {"command": "gemini", "args": ["-m", "gemini-2.5-pro"]},
    "gemini-2.5-flash":       {"command": "gemini", "args": ["-m", "gemini-2.5-flash"]}
  }
}
CONF
echo "OK: config written to $CONFIG_FILE"
```

8. **Verify installation**

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

# Verify hook
test -x "$HOME/.claude/hooks/subagent-code-routing.sh" && echo "OK: hook executable" || echo "ERROR: hook missing or not executable"
jq '.hooks.SubagentStart' "$HOME/.claude/settings.json"

# Verify config
cat ~/.config/oh-my-bridge/config.json | jq .routes
```

Expected output:
```text
OK: binary exists and is executable at .../oh-my-bridge
-rwxr-xr-x  ...  oh-my-bridge
---
name: oh-my-bridge:code-routing
description: ...
OK: hook executable
[{"hooks":[{"type":"command","command":"/Users/.../.claude/hooks/subagent-code-routing.sh","timeout":5}]}]
{
  "visual-engineering": "gemini-3-pro-preview",
  ...
}
```

9. **Report to user**

Tell the user:
- Skill installed to `~/.claude/skills/oh-my-bridge/SKILL.md`
- Slim routing rules installed to `~/.claude/skills/oh-my-bridge/code-routing-slim.md`
- SubagentStart hook installed to `~/.claude/hooks/subagent-code-routing.sh` — subagents will automatically inherit code-routing rules
- Config written to `~/.config/oh-my-bridge/config.json`
  - Routes (category → model) and model definitions are in the config
  - Edit the config to customize routing — `/oh-my-bridge:setup` resets it to defaults, so back up custom settings first
- `oh-my-bridge config` — 쉘 함수로 등록됨 (새 터미널 또는 `source ~/.zshrc` 후 사용 가능)
  - `oh-my-bridge config` — 카테고리별 모델 할당 TUI
  - `oh-my-bridge config list` — 현재 라우트 테이블 출력
  - `oh-my-bridge config validate` — config 검증
- **Restart Claude Code** for the skill to take effect
- After restart, Claude will automatically decide when to delegate and route to the best available model (Codex or Gemini) via the unified bridge MCP tool

## Undo

To remove the skills, run `/oh-my-bridge:uninstall`.
