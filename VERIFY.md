# oh-my-bridge 동작 확인 가이드

설치 후 각 레이어가 정상 동작하는지 확인하는 방법.

---

## 1. MCP 서버 연결 확인

Claude Code 세션에서:

```
/mcp
```

**정상:** 목록에 `bridge · ✔ connected` 표시

**이상:** `disconnected` 또는 목록에 없음 → `/oh-my-bridge:setup` 재실행 후 Claude Code 재시작

---

## 2. SubAgent 등록 확인

```
/agents
```

**정상:** Plugin agents 항목에 `oh-my-bridge:code-orchestrator · haiku` 표시

---

## 3. Skill 설치 확인

```bash
head -3 ~/.claude/skills/oh-my-bridge/SKILL.md
```

**정상 출력:**
```
---
name: oh-my-bridge:code-routing
description: ALWAYS invoke before any code change...
```

미설치 시: Claude Code에서 `/oh-my-bridge:setup` 실행 후 재시작.

---

## 4. Hook 로그 확인

bridge MCP를 호출한 적 있다면:

```bash
tail -1 ~/.claude/logs/codex-usage.log | jq .
```

**정상 출력 예시:**
```json
{
  "timestamp": "2026-03-03T08:00:00Z",
  "tool": "mcp__bridge__delegate",
  "status": "success",
  "exit_code": "",
  "error": ""
}
```

처음 설치 직후에는 파일이 없다. bridge(delegate)를 한 번 호출하면 생성된다.

---

## 5. E2E 흐름 확인

### 코드 생성 — MCP 위임 확인

Claude Code 세션에서:

```
/tmp/hello-bridge.js 에 hello world 함수를 작성해줘
```

**정상 실행 화면:**
```
⏺ mcp__bridge__delegate(...)
  ⎿  Done (...)
```

확인 항목:
1. `mcp__bridge__delegate` 도구가 호출됨 (Edit/Write 아닌)
2. UI에 "Error" 문구 없음
3. 로그 항목 추가 확인: `tail -1 ~/.claude/logs/codex-usage.log | jq .`

### 단순 편집 — 직접 처리 확인

```
README.md 첫 줄 오타 수정해줘
```

**정상:** Claude가 Edit 도구를 직접 사용 (MCP 미호출)

---

## 빠른 상태 점검 (한 번에)

```bash
echo "=== Skill ===" && \
  head -3 ~/.claude/skills/oh-my-bridge/SKILL.md 2>/dev/null || echo "(미설치 — /oh-my-bridge:setup 실행 필요)" && \
echo "=== Hook Log ===" && \
  tail -1 ~/.claude/logs/codex-usage.log 2>/dev/null | jq . || echo "(아직 로그 없음)"
```

MCP 연결과 SubAgent는 Claude Code 내에서 `/mcp`, `/agents`로 직접 확인.

---

## 문제 발생 시

| 증상 | 원인 | 해결 |
|------|------|------|
| MCP disconnected | 바이너리 없거나 경로 오류 | `/oh-my-bridge:setup` 재실행 후 Claude Code 재시작 |
| code-orchestrator 없음 | 플러그인 미설치 | `/plugin install oh-my-bridge` |
| Skill 없음 | setup 미실행 | `/oh-my-bridge:setup` 후 재시작 |
| 로그 파일 없음 | Codex 미호출 (정상) | E2E 테스트 한 번 실행 |
| Claude가 MCP 미호출 | Skill 미로드 | Claude Code 재시작 확인 |
