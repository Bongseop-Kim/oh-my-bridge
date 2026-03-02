# oh-my-bridge — CLAUDE.md

코드 파일 Edit|Write 시 자동으로 Codex CLI가 대신 실행된다.

## 전제조건

```bash
npm install -g @openai/codex
brew install jq
codex --version  # 설치 확인
```

## 핵심 명령어

```bash
# 스킬 오버라이드 배포 (/plugin install 이후 별도 실행 필요)
./setup.sh
./setup.sh --undo

# 재배포 순서
# 1. .claude-plugin/plugin.json 버전 수정
# 2. .claude-plugin/marketplace.json 버전 수정 (2곳: metadata.version, plugins[0].version)
# 3. git commit
# 4. Claude Code에서: /plugin update oh-my-bridge
# 5. Claude Code 재시작

# 캐시 직접 동기화 (버전 업 전 급할 때, 현재 버전: 1.0.4)
cp hooks/codex-interceptor.sh ~/.claude/plugins/cache/oh-my-bridge/oh-my-bridge/1.0.4/hooks/
cp hooks/hooks.json ~/.claude/plugins/cache/oh-my-bridge/oh-my-bridge/1.0.4/hooks/

# 사용 로그
tail -5 ~/.claude/logs/codex-usage.log | jq .
```

## Gotchas

- **훅은 세션 시작 시 스냅샷 로드** — 수정 후 Claude Code 재시작 필요
- **Edit 프롬프트는 `echo` 방식** — `printf` 쓰면 `%s` `%d` 포맷 문자 오해석
- **Write는 훅이 직접 파일 기록** — Codex 프롬프트 경유 없음 (Claude가 이미 내용 결정)
- **`/plugin install`은 `skills/` 미배포** — `setup.sh` 별도 실행 필요
- **마켓플레이스는 로컬 디렉토리** — GitHub push 불필요, `/plugin update`만으로 충분

## 동작 확인

```bash
/mcp     # plugin:oh-my-bridge:codex · ✔ connected
/agents  # oh-my-bridge:codex-generator · haiku
# 코드 파일 편집 시 "Routing to Codex CLI..." 스피너 확인
```
