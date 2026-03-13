---
name: oh-my-bridge:code-routing (subagent)
description: Slim routing rules for subagents — delegate logic changes to mcp__bridge__delegate
type: subagent
---

# Code Routing (subagent)

## Routing Rules

Before any Write/Edit call, decide:
- Logic changes (new functions/classes, refactoring, business logic) → delegate via `mcp__bridge__delegate`
- Simple edits (typos, config, docs, className, constants) → Claude handles directly

## How to Delegate

```
mcp__bridge__delegate({ prompt, category, cwd })
```

Category options:
- `visual-engineering`: UI/CSS/SVG/layout
- `ultrabrain`: algorithms/complex architecture
- `deep`: refactoring/multi-file logic
- `artistry`: creative patterns/design
- `quick`: boilerplate/scaffolding
- `writing`: docs/README
- `unspecified-high` / `unspecified-low`: when unclear (default: high)

Prompt uses 7-section format:
**TASK, EXPECTED OUTCOME, CONTEXT** (file paths only, no inline code),
**CONSTRAINTS, MUST DO, MUST NOT DO, OUTPUT FORMAT**

When in doubt → delegate. Do not default to handling directly.

## Post-processing

After delegation, confirm generated files with Read, report `{model} · {time}s · success`.
On `action:"claude"` response, Claude handles directly.
