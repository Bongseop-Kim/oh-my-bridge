---
name: subagent-driven-development
description: Use when executing implementation plans with independent tasks in the current session
---

# Subagent-Driven Development (oh-my-bridge override)

> **Override notice**: This skill overrides the Superpowers `subagent-driven-development` skill.
> Implementer is replaced with `codex-generator` SubAgent (GPT-5.3-codex via Codex CLI).
> Spec Reviewer and Code Quality Reviewer remain unchanged from the original.

## Trigger

Use this skill when the user asks to implement a feature, fix a bug, or generate code that requires writing new files or modifying existing ones.

Keywords: implement, build, create, write, add feature, fix bug, generate code

## Workflow

For each task, dispatch SubAgents in the following order:

### 1. Implementer — codex-generator SubAgent

Delegate code generation to the `codex-generator` SubAgent. Use the 7-Section delegation format defined in `implementer-prompt.md`.

```
Task agent: codex-generator
```

- The codex-generator SubAgent calls Codex CLI (`codex -q -a full-auto`) internally
- Actual code generation is performed by GPT-5.3-codex
- The SubAgent returns a file list and verification result

**Fallback strategy**: If codex-generator fails (non-zero exit, missing files, or syntax error), the `codex-fallback.sh` hook injects a fallback instruction. On receiving it, re-dispatch the same task to a Claude-native Implementer SubAgent instead.

**Worktree isolation (recommended)**: Run the Implementer in a git worktree to prevent unintended overwrites. Use the `using-git-worktrees` Superpowers skill before dispatching.

### 2. Spec Reviewer

Dispatch the original Spec Reviewer SubAgent (unchanged from Superpowers original).

Verify that the implementation matches the original specification:
- All acceptance criteria met
- No scope creep
- Edge cases handled

### 3. Code Quality Reviewer

Dispatch the original Code Quality Reviewer SubAgent (unchanged from Superpowers original).

Review code quality:
- Follows project conventions
- No obvious bugs or security issues
- Tests present where required

## Constraints

- Implementer must NOT be dispatched more than 3 times for the same task
- Each retry must include previous attempt history and error details (Stateless retry protocol)
- File changes per task: maximum 10 files (escalate to user if more are needed)
- Never pass secrets or credentials to codex-generator

## Notes

- This file is placed in `~/.claude/skills/subagent-driven-development/` to override the Superpowers original via personal skill priority matching
- `skills/` inside the plugin is NOT auto-deployed by `/plugin install` — use `setup.sh` or place manually
