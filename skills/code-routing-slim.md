---
name: oh-my-bridge:code-routing (subagent)
description: "You MUST use this before any Write/Edit that creates or changes logic. Routes to Codex (backend/logic), Gemini (UI/visual), or Claude-native (trivial edits) via mcp__bridge__delegate. Invoke for: new files, functions/classes, features, refactoring, test suites. Trigger on: 만들어줘/생성해줘/작성해줘/짜줘/개발해줘/구현해줘/적용해줘/추가해줘/넣어줘/붙여줘/리팩토링해줘/분리해줘/재설계해줘/뽑아줘/최적화해줘/개선해줘/정리해줘/설계해줘/잡아줘 · make/create/build/write/generate/implement/develop/add/insert/include/refactor/restructure/redesign/extract/optimize/improve/enhance/design/architect. Skip for: typos, config, docs, lock files."
type: subagent
---

# Code Routing (subagent)

## Routing Rules

Before any Write/Edit call, decide:

- Logic changes (new functions/classes, refactoring, business logic) → delegate via `mcp__bridge__delegate`
- Simple edits (typos, config, docs, className, constants) → Claude handles directly

## How to Delegate

```text
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
