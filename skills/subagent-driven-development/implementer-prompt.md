# Implementer Prompt — codex-generator Delegation Template

This file defines the prompt template used by the `codex-generator` SubAgent when delegating code generation to Codex CLI (GPT-5.3-codex).

## Dispatch instruction

When the skill dispatches the Implementer, send this to the `codex-generator` SubAgent:

```
Implement the following task using the 7-Section format below.
Construct the full delegation prompt and run Codex CLI with it.

--- DELEGATION PROMPT ---
1. TASK: {one-sentence atomic goal}
2. EXPECTED OUTCOME: {success criteria — what files exist, what tests pass, what behavior is observable}
3. CONTEXT: {current state of the codebase, relevant file paths, code snippets, background}
4. CONSTRAINTS: {tech stack, existing patterns to follow, items that must not change}
5. MUST DO:
   - {required action 1}
   - {required action 2}
6. MUST NOT DO:
   - {forbidden action 1}
   - {forbidden action 2}
7. OUTPUT FORMAT: {file list, test results, summary}
--- END DELEGATION PROMPT ---
```

## Codex CLI execution

The SubAgent runs:

```bash
codex -q -a full-auto --writable-roots "$(pwd)" "{assembled delegation prompt}"
```

## Stateless retry protocol

Because each Codex CLI invocation is stateless, retries must carry full history:

**Attempt 2+ template:**

```
1. TASK: {same as attempt 1}
2. EXPECTED OUTCOME: {same as attempt 1}
3. CONTEXT: {original context}
   --- PREVIOUS ATTEMPT HISTORY ---
   Attempt 1:
     Action taken: {what Codex did}
     Error: {exact error message or exit code}
   --- END HISTORY ---
4. CONSTRAINTS: {same + do not repeat the failed approach from attempt 1}
5. MUST DO: {same}
6. MUST NOT DO: {same + do not repeat: {failed approach}}
7. OUTPUT FORMAT: {same}
```

Maximum 3 attempts before escalating to the parent session.

## Worktree isolation

Before dispatching codex-generator for file-modifying tasks, the parent session should create a git worktree:

```bash
# Create isolated worktree for Codex work
git worktree add .worktrees/codex-impl -b feat/codex-impl-{task-id}
cd .worktrees/codex-impl
```

After Implementer success and reviewer approval, merge back:

```bash
git checkout main
git merge feat/codex-impl-{task-id}
git worktree remove .worktrees/codex-impl
```
