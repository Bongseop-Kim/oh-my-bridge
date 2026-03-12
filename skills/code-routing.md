---
name: oh-my-bridge:code-routing
description: "Multi-model router — classifies tasks and delegates to the best available model (Codex, Gemini, GPT-5.4, or Claude-native) via CLI without API keys. Invoke before any code change."
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

Before calling `mcp__bridge__delegate`, classify the task and select the appropriate model from the fallback chain below.

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

### Fallback Chain

Work through the chain top to bottom. Stop at the first success.

| Category | 1st | 2nd | 3rd |
|----------|-----|-----|-----|
| `visual-engineering` | Gemini Pro (high) | Claude (직접) | — |
| `ultrabrain` | GPT-5.3 Codex (xhigh) | Gemini Pro (high) | Claude (직접) |
| `deep` | GPT-5.3 Codex (medium) | Claude (직접) | Gemini Pro (high) |
| `artistry` | Gemini Pro (high) | Claude (직접) | GPT-5.4 |
| `quick` | Claude (직접) | Gemini Flash | GPT-5-Nano |
| `writing` | Gemini Flash | Claude (직접) | — |
| `unspecified-high` | GPT-5.4 (high) | Claude (직접) | — |
| `unspecified-low` | Claude (직접) | GPT-5.3 Codex (medium) | Gemini Flash |

### MCP Tool Mapping

All external models are called via `mcp__bridge__delegate`.

| Model | `model` param | `reasoning_effort` |
|-------|---------------|--------------------|
| GPT-5.3 Codex (xhigh) | `gpt-5.3-codex` | `high` |
| GPT-5.3 Codex (medium) | `gpt-5.3-codex` | `medium` |
| GPT-5.4 (high) | `gpt-5.4` | `high` |
| GPT-5-Nano | `gpt-5-nano` | — |
| Gemini Pro (high) | `gemini-2.5-pro` | — |
| Gemini Flash | `gemini-2.5-flash` | — |
| **Claude (직접)** | — | Edit/Write directly (no MCP) |

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
  model: "<model param from table above>",
  cwd: "<absolute project path>",
  reasoning_effort: "<effort if applicable, omit otherwise>"
})
```

## After delegation

1. Use `Read` to verify generated files exist and look correct.
2. Report to the user: file list + key decisions made + model used + fallback path (if any).

```yaml
category: deep
model used: GPT-5.3 Codex (medium)
fallback: none
```

3. If MCP fails: move to the next model in the fallback chain. Do not retry the same model.

## Security

- Never pass secrets, API keys, or credentials in the prompt.
- `cwd` must be the project directory — never `/`, `~`, or `$HOME`.
- **`bypassApprovals` is dangerous**: only set when `cwd` is an isolated, trusted workspace (e.g., CI sandbox or dedicated git worktree). Never set when `cwd` is a shared or production directory.
