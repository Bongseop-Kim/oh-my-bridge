# oh-my-bridge — CLAUDE.md

Claude가 코드 생성 작업을 자율 판단하여 Codex CLI(MCP)에 위임하는 skill 기반 브리지 플러그인 (v2.0.4).

## 전제조건

```bash
npm install -g @openai/codex
brew install jq
codex --version  # 설치 확인
```

## 핵심 명령어

```bash
# skill 설치 (Claude Code에서 실행)
/oh-my-bridge:setup

# skill 제거
/oh-my-bridge:uninstall

# 재배포 순서
# 1. ./bump-version.sh <new-version>  # 3개 파일 한 번에 업데이트
# 2. git commit
# 3. Claude Code에서: /plugin update oh-my-bridge
# 4. Claude Code 재시작

# 캐시 직접 동기화 (버전 업 전 급할 때, 현재 버전: 2.0.4)
cp skills/code-routing.md ~/.claude/plugins/cache/oh-my-bridge/oh-my-bridge/2.0.4/skills/code-routing.md

# 사용 로그
tail -5 ~/.claude/logs/codex-usage.log | jq .
```

## Skill 동작 방식

`/oh-my-bridge:setup` 실행 후 Claude는 `~/.claude/skills/oh-my-bridge/SKILL.md`를 세션마다 자동으로 읽는다.

- **코드 생성 작업** (새 파일, 함수/클래스 구현, 리팩토링) → `mcp__plugin_oh-my-bridge_codex__codex` 호출
- **단순 편집** (오타, 한 줄 변경, config, 문서) → Claude 네이티브 Edit/Write 직접 사용

MCP 호출 후에는 `Read` 도구로 생성 파일을 확인하고 결과를 보고한다.

## Gotchas

- **Skill은 세션 시작 시 로드** — 설치 후 Claude Code 재시작 필요
- **마켓플레이스는 로컬 디렉토리** — GitHub push 불필요, `/plugin update`만으로 충분
- **`/plugin install`은 `skills/` 미배포** — `/oh-my-bridge:setup` 별도 실행 필요

## MCP Latency

MCP 툴(Gemini, Codex)은 호출마다 새 프로세스를 spawn하고 LLM tool call 왕복이 추가되어 파일 생성 기준 20–30초 소요된다.

- Process cold start: ~3s
- Tool call additional API round trip (file write): +15–20s

Claude 네이티브 Write/Edit은 ~7s로 3–4배 빠르다. 단순 편집은 MCP를 거치지 않는 이유다.

## 동작 확인

```bash
/mcp     # plugin:oh-my-bridge:codex · ✔ connected
/agents  # oh-my-bridge:code-orchestrator · haiku
head -3 ~/.claude/skills/oh-my-bridge/SKILL.md  # name: oh-my-bridge:code-routing
```
