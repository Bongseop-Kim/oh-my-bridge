#!/bin/bash
# setup.sh — oh-my-bridge skill deployment helper
#
# Deploys the Phase 3 skill override to ~/.claude/skills/ so that
# Superpowers uses oh-my-bridge's codex-generator instead of the
# original Implementer SubAgent.
#
# Usage:
#   ./setup.sh          # deploy
#   ./setup.sh --undo   # restore backup (if available)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_SRC="${SCRIPT_DIR}/skills/subagent-driven-development"
SKILL_DST="${HOME}/.claude/skills/subagent-driven-development"
BACKUP_DST="${HOME}/.claude/skills/subagent-driven-development.bak"

# ── undo ──────────────────────────────────────────────────────────────────────
if [[ "${1:-}" == "--undo" ]]; then
  if [[ -d "$BACKUP_DST" ]]; then
    rm -rf "$SKILL_DST"
    mv "$BACKUP_DST" "$SKILL_DST"
    echo "Restored original skill from backup: $SKILL_DST"
  else
    echo "No backup found at $BACKUP_DST — nothing to restore."
    exit 1
  fi
  exit 0
fi

# ── deploy ────────────────────────────────────────────────────────────────────

# Ensure destination parent exists
mkdir -p "${HOME}/.claude/skills"

# Back up existing override if present
if [[ -d "$SKILL_DST" ]]; then
  if [[ -d "$BACKUP_DST" ]]; then
    echo "Removing old backup at $BACKUP_DST"
    rm -rf "$BACKUP_DST"
  fi
  echo "Backing up existing skill to $BACKUP_DST"
  cp -r "$SKILL_DST" "$BACKUP_DST"
fi

# Copy oh-my-bridge skill files
mkdir -p "$SKILL_DST"
cp "${SKILL_SRC}/SKILL.md" "$SKILL_DST/SKILL.md"
cp "${SKILL_SRC}/implementer-prompt.md" "$SKILL_DST/implementer-prompt.md"

# Copy reviewer prompts from Superpowers cache if available
SUPERPOWERS_CACHE=$(find "${HOME}/.claude/plugins/cache" -type d -name "superpowers" 2>/dev/null | head -1)
if [[ -n "$SUPERPOWERS_CACHE" ]]; then
  SP_SKILL_DIR=$(find "$SUPERPOWERS_CACHE" -type d -name "subagent-driven-development" 2>/dev/null | head -1)
  if [[ -n "$SP_SKILL_DIR" ]]; then
    for REVIEWER_PROMPT in spec-reviewer-prompt.md code-quality-reviewer-prompt.md; do
      if [[ -f "${SP_SKILL_DIR}/${REVIEWER_PROMPT}" ]]; then
        cp "${SP_SKILL_DIR}/${REVIEWER_PROMPT}" "${SKILL_DST}/${REVIEWER_PROMPT}"
        echo "Copied reviewer prompt from Superpowers: $REVIEWER_PROMPT"
      fi
    done
  fi
fi

echo ""
echo "✓ oh-my-bridge skill deployed to: $SKILL_DST"
echo ""
echo "Contents:"
ls -1 "$SKILL_DST"
echo ""
echo "To undo: ./setup.sh --undo"
echo "Note: Reviewer prompts not found in Superpowers cache will need to be"
echo "      copied manually from ~/.claude/plugins/cache/.../subagent-driven-development/"
