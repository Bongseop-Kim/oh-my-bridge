---
name: oh-my-bridge:model-routing
description: "Invoke after code-routing confirms delegation — classifies task category and selects the appropriate model via fallback chain."
---

# Model Routing

## Overview

After deciding to delegate code generation, use this skill to:
1. Classify the task into a category
2. Select the first available model in the fallback chain
3. Fall back to the next model if the current one fails

Claude is the orchestrator — it does not appear as an external MCP call. When the chain indicates **Claude (native)**, handle the task natively using Edit/Write tools.

---

## Category Classification

Classify the task before selecting a model. Pick the single best-matching category.

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

## Fallback Chain

Work through the chain top to bottom. Stop at the first success.

| Category | 1st | 2nd | 3rd | 4th | 5th |
|----------|-----|-----|-----|-----|-----|
| `visual-engineering` | Gemini Pro (high) | Claude (native) | — | — | — |
| `ultrabrain` | GPT-5.3 Codex (xhigh) | Gemini Pro (high) | Claude (native) | — | — |
| `deep` | GPT-5.3 Codex (medium) | Claude (native) | Gemini Pro (high) | — | — |
| `artistry` | Gemini Pro (high) | Claude (native) | GPT-5.4 | — | — |
| `quick` | Claude (native) | Gemini Flash | GPT-5-Nano | — | — |
| `writing` | Gemini Flash | Claude (native) | — | — | — |
| `unspecified-high` | GPT-5.4 (high) | Claude (native) | — | — | — |
| `unspecified-low` | Claude (native) | GPT-5.3 Codex (medium) | Gemini Flash | — | — |

---

## MCP Tool Mapping

All external models are called via a single unified MCP: `mcp__bridge__delegate`.

| Model | `model` param | `reasoning_effort` |
|-------|---------------|--------------------|
| GPT-5.3 Codex (xhigh) | `gpt-5.3-codex` | `high` |
| GPT-5.3 Codex (medium) | `gpt-5.3-codex` | `medium` |
| GPT-5.4 (high) | `gpt-5.4` | `high` |
| GPT-5-Nano | `gpt-5-nano` | — |
| Gemini Pro (high) | `gemini-2.5-pro` | — |
| Gemini Flash | `gemini-2.5-flash` | — |
| **Claude (native)** | — | Edit/Write directly (no MCP) |

---

## Latency Benchmark

| Method | Simple text response | File creation (tool call) |
|--------|---------------------|--------------------------|
| Claude native (Write) | — | ~7s |
| Gemini Flash (MCP) | ~8s | ~22s |
| Codex gpt-5.4 (MCP) | — | ~26s |



---

## Execution Flow

```text
1. Classify category from task description
2. Pick 1st model in chain
3. If MCP model:
   ├─ Call MCP tool with 7-Section prompt
   ├─ Success → done
   └─ Failure (error / timeout / unavailable) → move to next in chain
4. If Claude (native):
   └─ Handle natively with Edit/Write tools
5. After completion:
   └─ Report: category used, model used, fallback path taken (if any)
```

---

## Reporting Format

After every delegation, report to the user:

```yaml
category: deep
model used: GPT-5.3 Codex (medium)
fallback: none
```

If fallback occurred:

```yaml
category: ultrabrain
attempted: GPT-5.3 Codex (xhigh) → failed (MCP unavailable)
model used: Gemini Pro (high)
```
