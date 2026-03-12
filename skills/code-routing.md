---
name: oh-my-bridge:code-routing
description: "Multi-model router — classifies tasks and delegates to the best available model (Codex, Gemini, or Claude-native) via CLI without API keys. Invoke before any code change."
---

# Multi-Model Code Routing

## Why multi-model routing

Each model has a distinct personality suited for different tasks. All external models run as CLI tools — no API key setup required.

| Model | Personality | Best for |
|-------|-------------|----------|
| Claude | Mechanics-driven | Orchestration, trivial edits, direct simple tasks |
| Codex (GPT) | Principle-driven | Logic-heavy code, refactoring, complex business logic |
| Gemini Pro | Vision-driven | UI/UX, visual components, layout, design systems |
| Gemini Flash | Speed-driven | Documentation, boilerplate, fast turnaround |
| GPT-5.4 | Balanced | High-impact tasks where category is unclear |

Claude orchestrates — external models generate.

---

## Routing rule: output is code → delegate to external model

**Delegate to external model** when the result of the task is runnable code:

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

**When in doubt: delegate.** Claude's natural tendency is to handle things directly — resist it. The default for anything touching logic is external model delegation.

---

## After ExitPlanMode

Before executing the first step of an approved plan, apply the routing rules above to each step and decide: external model or direct?

Do not follow the plan's implicit implementation assumptions — plans are written before this skill is consulted. Re-evaluate each code-generating step now.

---

## Model Routing

Before calling `mcp__bridge__delegate`, classify the task and set the `category` field. The binary resolves the model from `~/.config/oh-my-bridge/config.json`.

### Category Classification

Pick the single best-matching category:

| Category | When to use |
|----------|------------|
| `visual-engineering` | UI components, CSS, SVG, layout, animation, design systems |
| `ultrabrain` | Algorithm design, complex architecture, mathematical optimization, deep reasoning |
| `deep` | Refactoring, multi-file logic changes, complex business logic |
| `artistry` | Creative patterns, expressive code style, novel design approaches |
| `quick` | Boilerplate, simple functions, stubs, scaffolding |
| `writing` | Documentation, comments, README, changelogs |
| `unspecified-high` | Unclear category, but high complexity or high impact |
| `unspecified-low` | Unclear category, low complexity or low impact |

**When in doubt between `unspecified-high` and `unspecified-low`:** prefer `unspecified-high`.

---

## How to delegate

Use the 7-Section format:

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

External models run with `workspace-write` sandbox and read files directly. **Do not embed file contents inline.**

```
# ❌ BAD — bloated prompt, poor readability, parsing artifacts
3. CONTEXT:
=== pages/login.tsx (full content) ===
import { useLogin } from "@refinedev/core";
... (300 lines)

# ✅ GOOD — model reads files autonomously
3. CONTEXT:
- apps/admin/src/pages/login.tsx — fat page to extract from
- apps/admin/src/features/claims/api/claims-mapper.ts — reference mapper pattern
- packages/shared/src/types/dto/admin-order.ts — AdminOrderListRowDTO type
```

Exception: paste short type definitions inline when field-level accuracy is critical (e.g., strict TypeScript contracts).

```
mcp__bridge__delegate({
  prompt: "<7-Section delegation prompt>",
  category: "<category from classification above>",
  cwd: "<absolute project path>",
  reasoning_effort: "<effort if applicable, omit otherwise>",
  model: "<optional model override — omit to use config routes>"
})
```

## After delegation

1. Use `Read` to verify generated files exist and look correct.
2. Report a one-line summary:

   - 정상 응답: **`{model} · {latency_ms/1000}s · success`** (예: `gpt-5.3-codex · 23s · success`)
   - `action: claude` 응답: **`claude · direct`**

   Then report: file list + key decisions made.

3. If response `action` is `"claude"`: handle the task directly with Claude native Edit/Write.
4. If MCP call fails: handle the task directly with Claude native Edit/Write. Do not retry.

## Security

- Never pass secrets, API keys, or credentials in the prompt.
- `cwd` must be the project directory — never `/`, `~`, or `$HOME`.
- **`bypassApprovals` is dangerous**: only set when `cwd` is an isolated, trusted workspace (e.g., CI sandbox or dedicated git worktree). Never set when `cwd` is a shared or production directory.
