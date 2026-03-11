---
name: oh-my-bridge:code-routing
description: "ALWAYS invoke before any code change — routes between Codex (logic/new code) and Claude-native (trivial edits). Do not skip regardless of context length."
---

# Code Generation Routing

## Why Codex for code, Claude for orchestration

This is not about capability — it's about model personality:

| Model | Personality | Prompt style |
|-------|-------------|--------------|
| Claude | Mechanics-driven | Detailed checklists, step-by-step procedures, "do exactly this" |
| Codex (GPT) | Principle-driven | Concise goals, autonomous execution, "achieve this, figure out how" |

Code generation is Codex's domain. Claude's role is to translate user intent into a precise prompt, then verify the result. Claude orchestrates — Codex generates.

---

## Routing rule: output is code → delegate to Codex

**Delegate to Codex** when the result of the task is runnable code:

- New file with any logic
- New function, class, method, or module
- Refactoring (restructuring, reorganizing, logic changes)
- Boilerplate generation (tests, stubs, scaffolding, API handlers)
- Adding or modifying logic in an existing file

**Handle directly** when the change contains no logic:

- Typo, wording, or comment fix
- Config value, constant, or version number update
- Documentation or Markdown edit
- Deleting or moving files

**When in doubt: delegate.** Claude's natural tendency is to handle things directly — resist it. The default for anything touching logic is Codex.

---

## After ExitPlanMode

Before executing the first step of an approved plan, apply the routing rules above to each step and decide: Codex or direct?

Do not follow the plan's implicit implementation assumptions — plans are written before this skill is consulted. Re-evaluate each code-generating step now.

---

## How to delegate

**Before calling any MCP tool, invoke `oh-my-bridge:model-routing` to classify the task category and select the appropriate model.** Do not default to Codex — the model is determined by the routing skill.

Use the 7-Section format and call the MCP tool returned by `oh-my-bridge:model-routing`:

```
1. TASK: [One atomic, specific goal sentence]
2. EXPECTED OUTCOME: [Success criteria]
3. CONTEXT: [Current state, relevant file paths/snippets, background]
4. CONSTRAINTS: [Tech constraints, existing patterns, must-not-change items]
5. MUST DO: [Required actions]
6. MUST NOT DO: [Forbidden actions]
7. OUTPUT FORMAT: [File paths, format, naming conventions]
```

### CONTEXT section: file paths, not inline content

Codex runs with `workspace-write` sandbox and reads files directly. **Do not embed file contents inline.**

```
# ❌ BAD — bloated prompt, poor readability, parsing artifacts
3. CONTEXT:
=== pages/login.tsx (full content) ===
import { useLogin } from "@refinedev/core";
... (300 lines)

# ✅ GOOD — Codex reads files autonomously
3. CONTEXT:
- apps/admin/src/pages/login.tsx — fat page to extract from
- apps/admin/src/features/claims/api/claims-mapper.ts — reference mapper pattern
- packages/shared/src/types/dto/admin-order.ts — AdminOrderListRowDTO type
```

Exception: paste short type definitions inline when field-level accuracy is critical (e.g., strict TypeScript contracts).

```
mcp__plugin_oh-my-bridge_codex__codex({
  prompt: "<7-Section delegation prompt>",
  cwd: "<absolute project path>",
  sandbox: "workspace-write",
  approval-policy: "never"
})
```

## After delegation

1. Use `Read` to verify generated files exist and look correct.
2. Report to the user: file list + key decisions made.
3. If MCP fails: follow the fallback chain in `oh-my-bridge:model-routing`. Do not retry the same model.

## Security

- Never pass secrets, API keys, or credentials in the prompt.
- `cwd` must be the project directory — never `/`, `~`, or `$HOME`.
