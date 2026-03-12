# oh-my-bridge — CLAUDE.md

Claude가 코드 생성 작업을 자율 판단하여 적합한 AI 모델(Codex/Gemini)에 위임하는 skill 기반 브리지 플러그인 (v2.2.0).

## 작업 관점 기준 (CRITICAL)

이 repo의 작업은 두 관점이 공존한다. **문제를 받았을 때 반드시 먼저 관점을 판단하라.**

| 관점 | 언제 | 경로 기준 |
|------|------|----------|
| **사용자** | MCP 연결 실패, 플러그인 동작 문제, 설치 후 에러 | `~/.claude/plugins/cache/oh-my-bridge/oh-my-bridge/<version>/` |
| **개발자** | 소스 코드 수정, 기능 추가, 버그 수정 | `/Users/duegosystem/git/oh-my-bridge/` |

**규칙:**
- MCP 연결 실패 → **사용자 관점** → 캐시 경로에서 `/oh-my-bridge:setup` 재실행
- 소스 변경 후 배포 → **개발자 관점** → repo에서 빌드 후 `bump-version + /plugin update`
- 절대로 사용자 문제를 개발자 repo 빌드로 해결하지 말 것

## 전제조건

```bash
npm install -g @openai/codex
npm install -g @google/gemini-cli
codex --version  # 설치 확인
gemini --version
```

## 핵심 명령어

```bash
# skill 설치 (Claude Code에서 실행)
/oh-my-bridge:setup

# skill 제거
/oh-my-bridge:uninstall

# 재배포 순서
# 1. ./bump-version.sh <new-version>  # 버전 업데이트 + commit + local tag
# 2. git push origin <branch> → PR → main 머지
# 3. git push origin v<new-version>   # GitHub Actions 트리거
# 4. (2분 대기) Claude Code에서: /plugin update oh-my-bridge
# 5. Claude Code 재시작

# 캐시 직접 동기화 (버전 업 전 급할 때, 현재 버전: 2.2.0)
cp skills/code-routing.md ~/.claude/plugins/cache/oh-my-bridge/oh-my-bridge/2.2.0/skills/code-routing.md

# 사용 로그
tail -5 ~/.claude/logs/codex-usage.log | jq .
```

## Skill 동작 방식

`/oh-my-bridge:setup` 실행 후 Claude는 `~/.claude/skills/oh-my-bridge/SKILL.md`를 세션마다 자동으로 읽는다.

- **코드 생성 작업** (새 파일, 함수/클래스 구현, 리팩토링) → `mcp__bridge__delegate` 호출 (model은 code-routing skill의 Fallback Chain이 결정)
- **단순 편집** (오타, 한 줄 변경, config, 문서) → Claude 네이티브 Edit/Write 직접 사용

MCP 호출 후에는 `Read` 도구로 생성 파일을 확인하고 결과를 보고한다.

## Gotchas

- **Skill은 세션 시작 시 로드** — 설치 후 Claude Code 재시작 필요
- **마켓플레이스는 로컬 디렉토리** — GitHub push 불필요, `/plugin update`만으로 충분
- **`/plugin install`은 `skills/` 미배포** — `/oh-my-bridge:setup` 별도 실행 필요

## MCP Latency

MCP 툴(Gemini, Codex)은 호출마다 새 프로세스를 spawn하고 LLM tool call 왕복이 추가되어 파일 생성 기준 20–30초 소요된다.

- Go 바이너리 cold start: ~3ms (Node.js 대비 ~260배 빠름)
- Tool call additional API round trip (file write): +15–20s

Claude 네이티브 Write/Edit은 ~7s로 3–4배 빠르다. 단순 편집은 MCP를 거치지 않는 이유다.

## 동작 확인

```bash
/mcp     # bridge · ✔ connected
/agents  # oh-my-bridge:code-orchestrator · haiku
head -3 ~/.claude/skills/oh-my-bridge/SKILL.md  # name: oh-my-bridge:code-routing
```
