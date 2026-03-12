<div align="center">

# oh-my-bridge

**Claude가 판단하고, 최적의 모델이 생성한다.**

Claude Code에서 작업 유형에 따라 외부 모델을 자동 선택하고 위임하는 브리지 플러그인.

[![GitHub Release](https://img.shields.io/github/v/release/Bongseop-Kim/oh-my-bridge?color=369eff&labelColor=black&logo=github&style=flat-square)](https://github.com/Bongseop-Kim/oh-my-bridge/releases)
[![License](https://img.shields.io/badge/license-MIT-white?labelColor=black&style=flat-square)](./LICENSE)

</div>

---

Claude Code는 훌륭한 오케스트레이터다. 하지만 코드 생성만큼은 GPT-5 Codex가 낫다. UI는 Gemini Pro가 앞선다. 빠른 보일러플레이트는 Claude가 직접 처리하는 게 빠르다.

oh-my-bridge는 그 판단을 자동화한다. 작업 유형을 분류하고, 가장 적합한 모델에 위임하고, 실패하면 다음 모델로 자동 전환한다. Claude는 생각하고, 외부 모델이 만든다.

---

## 어떻게 동작하는가

```text
사용자 요청
  → [code-routing] 코드 작업인가?
      → YES: [model-routing] 카테고리 분류 → 모델 선택
          → MCP 호출 성공 → 결과 검증 → 사용자 보고
          → MCP 호출 실패 → 다음 모델로 Fallback
      → NO: Claude 네이티브 Edit/Write 직접 사용
```

두 개의 Skill이 이 흐름을 제어한다.

| Skill | 역할 | 결정 |
|-------|------|------|
| `code-routing` | 위임 여부 판단 | "결과가 실행 가능한 코드인가?" → delegate / direct |
| `model-routing` | 모델 선택 | 카테고리 분류 → Fallback Chain 실행 |

> **code-routing은 WHY/WHEN, model-routing은 HOW/WHICH.**
> 관심사를 분리해 각각 독립적으로 교체 가능하다.

---

## 설치

### 전제 조건

```bash
npm install -g @openai/codex      # Codex CLI
npm install -g @google/gemini-cli # Gemini CLI

codex --version   # 설치 확인
gemini --version
```

### Phase 1 — 플러그인 설치

```bash
/plugin install /path/to/oh-my-bridge
```

자동으로 처리됨:
- `.mcp.json` — bridge MCP 서버 등록
- `agents/code-orchestrator.md` — SubAgent 등록
- `hooks/hooks.json` — PostToolUse 로깅 훅 바인딩

### Phase 2 — Skill 설치

```text
/oh-my-bridge:setup
```

Claude Code를 재시작하면 `code-routing`, `model-routing` Skill이 자동 적용된다.

---

## 모델 라인업

| MCP 서버 | 커버 모델 | 방식 |
|---------|---------|------|
| **bridge (Go)** | GPT-5.3 Codex, GPT-5.4, GPT-5-Nano, Gemini Pro/Flash | Go 정적 바이너리 |
| **Claude (직접)** | — | MCP 없음, Claude 자신이 처리 |

---

## 카테고리별 Fallback Chain

작업을 분류하면 모델이 자동으로 결정된다. 호출 실패 시 다음 모델로 자동 전환.

| 카테고리 | 적용 작업 | 1순위 | 2순위 | 3순위 |
|---------|---------|------|------|------|
| `visual-engineering` | UI, CSS, SVG, 레이아웃 | Gemini Pro | Claude | — |
| `ultrabrain` | 알고리즘, 복잡한 아키텍처 | GPT-5.3 Codex (xhigh) | Gemini Pro | Claude |
| `deep` | 리팩토링, 복잡한 로직 | GPT-5.3 Codex (medium) | Claude | Gemini Pro |
| `artistry` | 창의적 패턴, 코드 스타일 | Gemini Pro | Claude | GPT-5.4 |
| `quick` | 보일러플레이트, 단순 함수 | Claude | Gemini Flash | GPT-5-Nano |
| `writing` | 문서, 주석, README | Gemini Flash | Claude | — |
| `unspecified-high` | 판단 어렵고 중요도 높음 | GPT-5.4 | Claude | — |
| `unspecified-low` | 판단 어렵고 중요도 낮음 | Claude | GPT-5.3 Codex | Gemini Flash |

---

## 검증

### MCP 연결

```text
/mcp
```

아래 항목 `✔ connected` 확인:
- `bridge`

### Skill 설치

```bash
ls ~/.claude/skills/oh-my-bridge/
# code-routing.md  model-routing.md
```

### E2E 테스트

**코드 생성 — MCP 위임 확인:**

```text
Express.js REST API 엔드포인트 구현해줘
```

1. Claude가 카테고리를 `deep` 또는 `unspecified-high`로 분류
2. `mcp__bridge__delegate` 호출 (model은 model-routing skill이 결정)
3. 응답에 `category`, `model used` 포함

**단순 편집 — 직접 처리 확인:**

```text
README.md 오타 수정해줘
```

Claude가 Edit 직접 사용. MCP 미호출.

---

## 로그

```bash
# 최근 5건
tail -5 ~/.claude/logs/codex-usage.log | jq .

# 에러만
jq 'select(.status == "error")' ~/.claude/logs/codex-usage.log

# 오늘 사용량
jq 'select(.timestamp | startswith("'"$(date -u +%Y-%m-%d)"'"))' ~/.claude/logs/codex-usage.log
```

---

## 디렉토리 구조

```text
oh-my-bridge/
├── .claude-plugin/
│   ├── marketplace.json
│   └── plugin.json
├── .mcp.json                 bridge MCP 등록
├── .goreleaser.yml           GoReleaser 릴리즈 자동화
├── .github/workflows/
│   └── release.yml          태그 push 시 바이너리 빌드 + 배포
├── mcp-servers/
│   └── bridge/              Go MCP 서버 (정적 바이너리)
│       ├── main.go
│       ├── go.mod
│       └── go.sum
├── agents/
│   └── code-orchestrator.md   SubAgent (카테고리 분류 + MCP 호출)
├── commands/
│   ├── setup.md             /oh-my-bridge:setup
│   └── uninstall.md         /oh-my-bridge:uninstall
├── hooks/
│   ├── hooks.json
│   └── log-codex-usage.sh
├── skills/
│   ├── code-routing.md      위임 여부 판단
│   └── model-routing.md     카테고리 분류 + Fallback Chain
└── bump-version.sh
```

---

## 개발

```bash
# 로컬 바이너리 빌드 (개발자용, Go 필요)
cd mcp-servers/bridge && CGO_ENABLED=0 go build -o oh-my-bridge .

# 버전 업데이트 + 릴리즈 (commit + tag + push → GitHub Actions 자동 빌드)
./bump-version.sh <new-version>

# 캐시 직접 동기화 (skill 파일만, 급할 때)
cp skills/code-routing.md ~/.claude/plugins/cache/oh-my-bridge/oh-my-bridge/$(cat .claude-plugin/plugin.json | jq -r .version)/skills/code-routing.md

# 재배포 순서
# 1. ./bump-version.sh <version>   ← commit + local tag 자동 포함
# 2. git push origin <branch> → PR → main 머지
# 3. git push origin v<version>    ← GitHub Actions 트리거
# 4. (2분 대기) /plugin update oh-my-bridge
# 5. Claude Code 재시작
```

---

## 제거

```
/oh-my-bridge:uninstall
```

Skill 파일 제거 후 Claude Code를 재시작하면 플러그인이 비활성화된다.
