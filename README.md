# oh-my-bridge

Claude Code + Codex CLI 외부 모델 통합 브리지 플러그인.

**핵심 원칙: Claude가 판단하고, GPT가 생성한다.**

MCP → SubAgent → Hook → Skill → Plugin 레이어를 순서대로 쌓아 외부 모델(GPT-5.3-codex)을 Claude Code 워크플로우에 통합한다.

---

## 아키텍처

```
┌─────────────────────────────────────────┐
│  Skill (판단 규칙)                        │  "언제" 외부 모델을 쓸지 결정
├─────────────────────────────────────────┤
│  SubAgent (실행 오케스트레이션)              │  "어떻게" 호출하고 결과를 검증할지 제어
├─────────────────────────────────────────┤
│  MCP Server (도구 등록)                    │  Codex CLI를 네이티브 도구로 등록
├─────────────────────────────────────────┤
│  Hook (인터셉션 + 모니터링)                  │  코드 편집 자동 라우팅, 비용 로깅, 에러 감지, fallback
├─────────────────────────────────────────┤
│  Plugin (패키징)                           │  위 전체를 설치 가능한 단위로 번들링
└─────────────────────────────────────────┘
```

**실행 흐름 — 경로 A: 자동 인터셉션 (모든 코드 편집에 자동 적용)**

```
Claude → Edit|Write 시도
  → PreToolUse Hook (codex-interceptor.sh)
  → codex -q -a full-auto --writable-roots {cwd}
  → 성공: deny (Codex가 이미 수정) / 실패: allow (Claude 네이티브 폴백)
```

**실행 흐름 — 경로 B: 명시적 스킬 (리뷰 파이프라인 포함)**

```
/subagent-driven-development implement X
  → codex-generator SubAgent 디스패치 (haiku 오케스트레이터)
  → codex -q -a full-auto ... (GPT-5.3-codex 코드 생성)
  → 결과 검증
  → Spec Reviewer + Code Quality Reviewer (Claude 네이티브)
```

---

## 전제 조건

| 도구 | 설치 확인 |
|------|----------|
| Claude Code | `claude --version` |
| Codex CLI (`@openai/codex` ≥ v0.106.0) | `codex --version` |
| jq | `jq --version` |
| Superpowers (Phase 3 선택) | `/plugin list` 에서 확인 |

Codex CLI 설치:

```bash
npm install -g @openai/codex
```

MCP 서버 모드 확인:

```bash
codex mcp-server --help
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
- `hooks/hooks.json` — PreToolUse(코드 편집 인터셉션) + PostToolUse(로깅/fallback) 훅 바인딩

### Phase 3: 스킬 오버라이드 (Superpowers 필요)

`/plugin install`은 `skills/`를 `~/.claude/skills/`에 복사하지 않는다. 수동 배포 필요:

```bash
# setup.sh로 자동 배포
./setup.sh

# 되돌리기
./setup.sh --undo
```

또는 수동:

```bash
mkdir -p ~/.claude/skills/subagent-driven-development
cp skills/subagent-driven-development/SKILL.md ~/.claude/skills/subagent-driven-development/
cp skills/subagent-driven-development/implementer-prompt.md ~/.claude/skills/subagent-driven-development/
# Spec Reviewer, Code Quality Reviewer는 Superpowers 캐시에서 복사
```

---

## 디렉토리 구조

```
oh-my-bridge/
├── CLAUDE.md                          프로젝트 컨텍스트 (명령어, gotcha)
├── LICENSE
├── .claude-plugin/
│   └── plugin.json                    Phase 4: 플러그인 메타데이터
├── .mcp.json                          Phase 1a: Codex MCP 서버 등록
├── agents/
│   └── codex-generator.md             Phase 1b: SubAgent 정의
├── hooks/
│   ├── hooks.json                     Phase 2: Hook 이벤트 바인딩
│   ├── codex-interceptor.sh           Phase 2: PreToolUse 코드 편집 자동 인터셉션
│   ├── log-codex-usage.sh             Phase 2: JSONL 사용량 로깅
│   └── codex-fallback.sh              Phase 2: 장애 감지 + fallback 주입
├── skills/
│   └── subagent-driven-development/
│       ├── SKILL.md                   Phase 3: 워크플로우 오버라이드
│       └── implementer-prompt.md      Phase 3: 위임 프롬프트 템플릿
├── setup.sh                           스킬 배포 헬퍼
└── README.md
```

---

## 검증

### Phase 1a — MCP 도구 인식 확인

`/plugin install` 후 Claude Code 세션에서:

```
사용 가능한 도구 목록에 mcp__plugin_oh-my-bridge_codex__codex가 표시되는지 확인
```

### Phase 1b — SubAgent 등록 확인

```
에이전트 목록에 codex-generator가 표시되는지 확인
```

### Phase 2 — 훅 동작 확인

**PreToolUse 인터셉션 확인**: `.js`, `.ts`, `.py` 등 코드 파일을 편집 요청 시:

```
"Routing to Codex CLI..." 스피너가 표시되는지 확인
```

**PostToolUse 로그 확인**:

```bash
# 로그 확인
cat ~/.claude/logs/codex-usage.log

# 최근 항목 (jq 필요)
tail -5 ~/.claude/logs/codex-usage.log | jq .
```

의도적 에러 유발 시 fallback `additionalContext` 주입 확인.

### Phase 3 — 스킬 오버라이드 확인

```bash
# setup.sh 실행 후
cat ~/.claude/skills/subagent-driven-development/SKILL.md | head -5
# "oh-my-bridge override" 텍스트가 표시되면 성공
```

### E2E 테스트

**경로 A — 자동 인터셉션 확인**:

```
hello.js 파일에 hello world 함수를 추가해줘
```

확인 항목:
1. "Routing to Codex CLI..." 스피너 표시됨
2. Codex CLI가 파일을 직접 수정함 (Claude 네이티브 편집 없음)

**경로 B — 명시적 스킬 트리거 확인**:

```
/subagent-driven-development implement a hello world function in hello.js
```

확인 항목:
1. codex-generator SubAgent 디스패치됨
2. Codex CLI 실행됨 (GPT-5.3-codex 코드 생성)
3. `~/.claude/logs/codex-usage.log`에 항목 추가됨
4. Spec Reviewer + Code Quality Reviewer 순서대로 실행됨

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

---

## 참고

- [Part 5. Oh My Bridge 구현 가이드](../items/part5-oh-my-bridge.md)
- [claude-delegator](https://github.com/jarrodwatts/claude-delegator) — 검증된 Codex MCP 통합 패턴
