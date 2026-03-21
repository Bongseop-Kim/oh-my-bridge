---
name: oh-my-bridge:code-routing
description: "You MUST use this before writing any code that creates or changes logic — new components, pages, services, controllers, API handlers, test suites, algorithm implementations, or any refactoring. This is the multi-model routing gateway: routes to Codex (backend/logic), Gemini (UI/visual), or Claude-native (trivial edits) via mcp__bridge__delegate. Invoke before: creating new files, implementing functions or classes, adding features, refactoring modules, or optimizing algorithms. Also trigger when the user says — 생성(만들어줘/생성해줘/작성해줘/짜줘/개발해줘 · make/create/build/write/generate), 구현(구현해줘/적용해줘 · implement/develop), 추가(추가해줘/넣어줘/붙여줘 · add/insert/include), 재설계(리팩토링해줘/분리해줘/재설계해줘/뽑아줘 · refactor/restructure/redesign/extract), 개선(최적화해줘/개선해줘/정리해줘 · optimize/improve/clean up/enhance), 설계(설계해줘/잡아줘 · design/architect). Skip only for: typo fixes, config values, doc edits, lock files, or explaining code."
---

# Multi-Model Code Routing

## Why multi-model routing

Each model has a distinct personality suited for different tasks. All external models run as CLI tools — no API key setup required.

| Model        | Personality      | Best for                                              |
| ------------ | ---------------- | ----------------------------------------------------- |
| Claude       | Mechanics-driven | Orchestration, trivial edits, direct simple tasks     |
| Codex (GPT)  | Principle-driven | Logic-heavy code, refactoring, complex business logic |
| Gemini Pro   | Vision-driven    | UI/UX, visual components, layout, design systems      |
| Gemini Flash | Speed-driven     | Documentation, boilerplate, fast turnaround           |
| GPT-5.4      | Balanced         | High-impact tasks where category is unclear           |

Claude orchestrates — external models generate.

---

## Before executing each plan step

For each step in an approved plan, decide **who executes it** — the plan defines _what_, this skill defines _who_:

- Step introduces or modifies logic → delegate to external model (apply category classification below)
- Step is a trivial edit, config, or doc change → Claude native Edit/Write

This is a routing decision only. Do not alter the plan's scope or goals.

---

## Routing rule: result contains logic → delegate to external model

**Delegate to external model** when the task introduces or changes executable logic:

- New file with any logic
- New function, class, method, or module
- Refactoring (restructuring, reorganizing, logic changes)
- Boilerplate generation (tests, stubs, scaffolding, API handlers)
- Adding or modifying logic in an existing file

**Handle directly** when the change carries no logic — only data, text, or structure:

- Typo, wording, or comment fix
- Config value, constant, or version number update
- Documentation or Markdown edit
- Deleting or moving files
- Tailwind className, style attribute, or CSS value change
- Auto-generated files (e.g. Supabase types, GraphQL schema, OpenAPI clients)
- Lock file updates (package-lock.json, yarn.lock, bun.lock)
- Asset file changes (images, fonts, icons, SVG assets)
- Environment variable additions (.env, .env.local)

**When in doubt: delegate.** Claude's natural tendency is to handle things directly — resist it. The default for anything touching logic is external model delegation.

---

## Model Routing

Before calling `mcp__bridge__delegate`, classify the task and set the `category` field. The binary resolves the model from `~/.config/oh-my-bridge/config.json`.

### Category Classification

Pick the single best-matching category:

| Category             | When to use                                                                       |
| -------------------- | --------------------------------------------------------------------------------- |
| `visual-engineering` | UI components, CSS, SVG, layout, animation, design systems                        |
| `ultrabrain`         | Algorithm design, complex architecture, mathematical optimization, deep reasoning |
| `deep`               | Refactoring, multi-file logic changes, complex business logic                     |
| `artistry`           | Creative patterns, expressive code style, novel design approaches                 |
| `quick`              | Boilerplate, simple functions, stubs, scaffolding                                 |
| `writing`            | Documentation, comments, README, changelogs                                       |
| `unspecified-high`   | Unclear category, but high complexity or high impact                              |
| `unspecified-low`    | Unclear category, low complexity or low impact                                    |

**When in doubt between `unspecified-high` and `unspecified-low`:** prefer `unspecified-high`.

---

## How to delegate

Use the 7-Section format:

```text
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

```text
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

```text
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
   Check the `reason` field to understand why and adjust behavior:
   - `route_configured`: route explicitly set to "claude" — handle directly as normal
   - `cli_not_installed`: CLI binary missing — handle directly, optionally notify user
   - `cli_error_timeout`: CLI timed out — handle directly; CLI was too slow for this task
   - `cli_error_rate_limit`: API rate limit hit — handle directly; optionally advise user to retry later
   - `cli_error_crash`: CLI exited non-zero — handle directly; consider checking CLI health

4. If MCP call fails, check the error before falling back:
   - If error contains `"cwd must stay within workspace root"`:
     - `OH_MY_BRIDGE_WORKSPACE_ROOT` is not set or points to the wrong directory
     - Verify the env var is set to the project root, or relaunch MCP server from the correct directory
     - Do NOT fall back to direct handling
   - If error contains `category %q not found in config routes`:
     - Config error — do NOT fall back to direct handling
     - Verify category spelling in `~/.config/oh-my-bridge/config.json` and retry
   - Otherwise (generic runtime error): handle the task directly with Claude native Edit/Write. Do not retry.

## Security

- Never pass secrets, API keys, or credentials in the prompt.
- `cwd` must be the project directory — never `/`, `~`, or `$HOME`.
- **`bypassApprovals` is dangerous**: only set when `cwd` is an isolated, trusted workspace (e.g., CI sandbox or dedicated git worktree). Never set when `cwd` is a shared or production directory.
