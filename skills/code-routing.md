---
name: oh-my-bridge:code-routing
description: Use when you are about to write code, create new files, or implement features. Determines whether to delegate code generation to Codex CLI via MCP or handle directly.
---

# Code Generation Routing

When you are about to write or modify code, apply these rules to decide whether to delegate to Codex CLI via MCP or handle it yourself.

---

## Delegate to Codex (call `mcp__plugin_oh-my-bridge_codex__codex`)

- Creating a new file with more than ~20 lines of logic
- Implementing a function, class, or module from scratch
- Refactoring an existing file (restructuring, renaming, reorganizing)
- Generating boilerplate (tests, stubs, scaffolding, API handlers)
- Writing algorithmic or business-logic code in any language

## Handle directly (use Edit/Write/Bash as normal)

- Typo or wording fix (1–3 lines)
- Updating a constant, version number, or config value
- Editing documentation or Markdown files
- Renaming a single variable across a small file
- Deleting or moving files

**Rule of thumb**: If you can write it correctly in a single Edit call, do it yourself. If it requires thinking through logic or structure, delegate.

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
