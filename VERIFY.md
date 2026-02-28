# oh-my-bridge 동작 확인 가이드

설치 후 각 레이어가 정상 동작하는지 눈으로 확인하는 방법.

---

## 1. MCP 서버 연결 확인

Claude Code 세션에서:

```
/mcp
```

**정상:** 목록에 `plugin:oh-my-bridge:codex · ✔ connected` 표시

**이상:** `disconnected` 또는 목록에 없음 → Codex CLI 설치 확인 (`codex --version`)

---

## 2. SubAgent 등록 확인

```
/agents
```

**정상:** Plugin agents 항목에 `oh-my-bridge:codex-generator · haiku` 표시

---

## 3. Hook 로그 확인

Codex MCP를 호출한 적 있다면:

```bash
cat ~/.claude/logs/codex-usage.log
```

**정상 출력 예시:**
```json
{
  "timestamp": "2026-02-28T08:59:54Z",
  "tool": "mcp__plugin_oh-my-bridge_codex__codex",
  "status": "success",
  "exit_code": "",
  "error": ""
}
```

처음 설치 직후에는 파일이 없다. Codex를 한 번 호출하면 생성된다.

**에러 항목 필터:**
```bash
jq 'select(.status == "error")' ~/.claude/logs/codex-usage.log
```

---

## 4. 스킬 오버라이드 확인

```bash
head -5 ~/.claude/skills/subagent-driven-development/SKILL.md
```

**정상 출력:**
```
---
name: subagent-driven-development
description: Use when executing implementation plans with independent tasks in the current session
---

# Subagent-Driven Development (oh-my-bridge override)
```

`oh-my-bridge override` 텍스트가 보이면 오버라이드 적용됨.

---

## 5. E2E 흐름 확인

Claude Code 세션에서:

```
/subagent-driven-development implement a hello world function in /tmp/hello-bridge.js
```

**정상 실행 화면:**
```
⏺ Step 1: Dispatching Implementer (codex-generator SubAgent)

⏺ oh-my-bridge:codex-generator(...)
  ⎿  Done (...)

⏺ Step 2 & 3: Dispatching Spec Reviewer and Code Quality Reviewer in parallel
...
```

`oh-my-bridge:codex-generator`가 Implementer로 디스패치되면 전체 흐름 정상.

실행 후 로그도 같이 확인:
```bash
tail -1 ~/.claude/logs/codex-usage.log | jq .
```

새 항목 추가 확인.

---

## 빠른 상태 점검 (한 번에)

```bash
echo "=== MCP ===" && \
  echo "Claude Code에서 /mcp 실행 후 plugin:oh-my-bridge:codex 확인" && \
echo "=== SubAgent ===" && \
  echo "Claude Code에서 /agents 실행 후 oh-my-bridge:codex-generator 확인" && \
echo "=== Skill Override ===" && \
  head -6 ~/.claude/skills/subagent-driven-development/SKILL.md && \
echo "=== Hook Log ===" && \
  tail -3 ~/.claude/logs/codex-usage.log 2>/dev/null || echo "(아직 로그 없음)"
```

---

## 문제 발생 시

| 증상 | 원인 | 해결 |
|------|------|------|
| MCP disconnected | Codex CLI 미설치 또는 인증 안 됨 | `codex --version`, `codex /status` 확인 |
| codex-generator 없음 | 플러그인 미설치 | `/plugin install oh-my-bridge` |
| 로그 파일 없음 | Codex 미호출 (정상) | E2E 테스트 한 번 실행 |
| 스킬 오버라이드 안 됨 | setup.sh 미실행 | `cd oh-my-bridge && ./setup.sh` |
| Implementer가 Claude 직접 실행 | SDD 스킬 미트리거 | `/subagent-driven-development` 명시적으로 앞에 붙이기 |
