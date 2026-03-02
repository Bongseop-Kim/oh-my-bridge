# oh-my-bridge

Claude Code + Codex CLI 외부 모델 통합 브리지 플러그인.

**핵심 원칙: Claude가 판단하고, GPT가 생성한다.**

Skill → SubAgent → MCP 레이어를 통해 외부 모델(GPT-5.3-codex)을 Claude Code 워크플로우에 통합한다.

---

## 아키텍처

```
┌─────────────────────────────────────────┐
│  Skill (라우팅 판단)                       │  "언제" 외부 모델을 쓸지 결정
├─────────────────────────────────────────┤
│  SubAgent (실행 오케스트레이션)              │  "어떻게" 호출하고 결과를 검증할지 제어
├─────────────────────────────────────────┤
│  MCP Server (도구 등록)                    │  Codex CLI를 네이티브 도구로 등록
├─────────────────────────────────────────┤
│  Hook (모니터링)                           │  MCP 호출 비용 로깅
├─────────────────────────────────────────┤
│  Plugin (패키징)                           │  위 전체를 설치 가능한 단위로 번들링
└─────────────────────────────────────────┘
```

**실행 흐름 — Skill 기반 자율 라우팅**

```
사용자 요청
  → Claude가 oh-my-bridge:code-routing 스킬 판단 기준 적용
  → 코드 생성 작업: mcp__plugin_oh-my-bridge_codex__codex 호출
      → Codex CLI (GPT-5.3-codex) 코드 생성
      → Claude가 Read로 결과 검증 → 사용자에게 보고
  → 단순 편집: Claude 네이티브 Edit/Write 직접 사용
```

---

## 전제 조건

| 도구 | 설치 확인 |
|------|----------|
| Claude Code | `claude --version` |
| Codex CLI (`@openai/codex` ≥ v0.106.0) | `codex --version` |
| jq | `jq --version` |

Codex CLI 설치:

```bash
npm install -g @openai/codex
```

---

## 설치

### Phase 1–2: 플러그인 설치

```bash
# 로컬 경로에서 설치
/plugin install /path/to/oh-my-bridge
```

설치 후 자동으로:
- `.mcp.json` — `mcp__plugin_oh-my-bridge_codex__codex` 도구 등록
- `agents/codex-generator.md` — SubAgent 자동 등록
- `hooks/hooks.json` — PostToolUse 로깅 훅 바인딩

### Phase 3: Skill 설치

`/plugin install`은 `skills/`를 `~/.claude/skills/`에 복사하지 않는다. 슬래시 커맨드로 배포:

```
/oh-my-bridge:setup
```

Claude Code를 재시작하면 `oh-my-bridge:code-routing` 스킬이 자동 적용된다.

---

## 디렉토리 구조

```
oh-my-bridge/
├── CLAUDE.md                          프로젝트 컨텍스트 (명령어, gotcha)
├── LICENSE
├── .claude-plugin/
│   ├── marketplace.json               마켓플레이스 메타데이터
│   └── plugin.json                    플러그인 메타데이터
├── .mcp.json                          Codex MCP 서버 등록
├── agents/
│   └── codex-generator.md             SubAgent 정의 (MCP 호출 오케스트레이션)
├── commands/
│   ├── setup.md                       /oh-my-bridge:setup 슬래시 커맨드
│   └── uninstall.md                   /oh-my-bridge:uninstall 슬래시 커맨드
├── hooks/
│   ├── hooks.json                     PostToolUse 로깅 훅
│   └── log-codex-usage.sh             JSONL 사용량 로깅
├── skills/
│   └── code-routing.md                설치용 스킬 (→ ~/.claude/skills/oh-my-bridge/SKILL.md)
├── bump-version.sh                    버전 업데이트 헬퍼
└── README.md
```

---

## 검증

### Phase 1a — MCP 도구 인식 확인

```
/mcp
```

`plugin:oh-my-bridge:codex · ✔ connected` 표시 확인.

### Phase 1b — SubAgent 등록 확인

```
/agents
```

`oh-my-bridge:codex-generator · haiku` 표시 확인.

### Phase 3 — Skill 설치 확인

```bash
head -3 ~/.claude/skills/oh-my-bridge/SKILL.md
# name: oh-my-bridge:code-routing
```

### E2E 테스트

**코드 생성 (MCP 위임 확인)**:

```
Express.js REST API 엔드포인트 구현해줘
```

확인 항목:
1. Claude가 `mcp__plugin_oh-my-bridge_codex__codex` 호출 (Edit 아닌)
2. 로그에 항목 추가됨: `tail -1 ~/.claude/logs/codex-usage.log | jq .`
3. UI에 "Error" 문구 없음

**단순 편집 (직접 처리 확인)**:

```
README.md 오타 수정해줘
```

확인 항목:
1. Claude가 Edit 직접 사용 (MCP 미호출)

---

## 로그 조회

```bash
# 전체 로그
cat ~/.claude/logs/codex-usage.log

# 에러만 필터
jq 'select(.status == "error")' ~/.claude/logs/codex-usage.log

# 오늘 사용량
jq 'select(.timestamp | startswith("'"$(date -u +%Y-%m-%d)"'"))' ~/.claude/logs/codex-usage.log
```
