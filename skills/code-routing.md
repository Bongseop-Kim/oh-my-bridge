---
name: oh-my-bridge:code-routing
description: Use when you are about to write code, create new files, or implement features. Determines whether to delegate code generation to Codex CLI via MCP or handle directly.
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

Use the 7-Section format and call `mcp__plugin_oh-my-bridge_codex__codex`:

```
1. TASK: [One atomic, specific goal sentence]
2. EXPECTED OUTCOME: [Success criteria]
3. CONTEXT: [Current state, relevant file paths/snippets, background]
4. CONSTRAINTS: [Tech constraints, existing patterns, must-not-change items]
5. MUST DO: [Required actions]
6. MUST NOT DO: [Forbidden actions]
7. OUTPUT FORMAT: [File paths, format, naming conventions]
```

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
3. If MCP fails: fall back to Claude-native Edit/Write. Do not retry Codex.

## Security

- Never pass secrets, API keys, or credentials in the prompt.
- `cwd` must be the project directory — never `/`, `~`, or `$HOME`.
