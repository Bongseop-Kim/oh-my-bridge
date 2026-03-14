# oh-my-bridge — CLAUDE.md

Claude가 코드 생성 작업을 자율 판단하여 적합한 AI 모델(Codex/Gemini)에 위임하는 skill 기반 브리지 플러그인 (v2.4.3).

## 작업 관점 기준 (CRITICAL)

이 repo의 작업은 두 관점이 공존한다. **문제를 받았을 때 반드시 먼저 관점을 판단하라.**

| 관점       | 언제                                            | 경로 기준                                                      |
| ---------- | ----------------------------------------------- | -------------------------------------------------------------- |
| **사용자** | MCP 연결 실패, 플러그인 동작 문제, 설치 후 에러 | `~/.claude/plugins/cache/oh-my-bridge/oh-my-bridge/<version>/` |
| **개발자** | 소스 코드 수정, 기능 추가, 버그 수정            | `/Users/duegosystem/git/oh-my-bridge/`                         |

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

# (선택) 린터 + pre-commit
brew install golangci-lint  # v1.64.x
go install github.com/evilmartians/lefthook@latest
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

# 캐시 직접 동기화 (버전 업 전 급할 때)
VERSION=$(jq -r '.version' .claude-plugin/plugin.json)
cp skills/code-routing.md ~/.claude/plugins/cache/oh-my-bridge/oh-my-bridge/${VERSION}/skills/code-routing.md

# config 확인/편집
cat ~/.config/oh-my-bridge/config.json | jq .

# 사용 로그
tail -5 ~/.claude/logs/oh-my-bridge.log | jq .
```

## Skill 동작 방식

`/oh-my-bridge:setup` 실행 후 Claude는 `~/.claude/skills/oh-my-bridge/SKILL.md`를 세션마다 자동으로 읽는다.

- **코드 생성 작업** (새 파일, 함수/클래스 구현, 리팩토링) → `mcp__bridge__delegate` 호출 (`category` 필수, 바이너리가 `~/.config/oh-my-bridge/config.json`에서 모델 결정)
- **단순 편집** (오타, 한 줄 변경, config, 문서) → Claude 네이티브 Edit/Write 직접 사용
- **claude 응답** (`action: "claude"`) → Claude가 직접 처리 (route가 claude이거나 CLI 미설치)

MCP 호출 후에는 `Read` 도구로 생성 파일을 확인하고 결과를 보고한다.

## Gotchas

- **Skill은 세션 시작 시 로드** — 설치 후 Claude Code 재시작 필요
- **마켓플레이스는 로컬 디렉토리** — GitHub push 불필요, `/plugin update`만으로 충분
- **`/plugin install`은 `skills/` 미배포** — `/oh-my-bridge:setup` 별도 실행 필요

## MCP Latency

MCP 툴(Codex, Gemini)은 파일 생성 기준 **20–30초** 소요된다. Claude 네이티브 Write/Edit은 ~7s로 3–4배 빠르다. 단순 편집은 MCP를 거치지 않는 이유다.

지연 원인과 프로세스 생명주기 상세: [docs/architecture.md](docs/architecture.md#3-mcp-서버-프로세스-생명주기)

## 코드 펜스 규칙

모든 코드 블록에는 반드시 언어를 명시한다.

```markdown
# ✅ 올바른 예

\`\`\`bash
echo "hello"
\`\`\`

# ❌ 잘못된 예

\`\`\`
echo "hello"
\`\`\`
```

언어를 알 수 없는 경우 `text`를 사용한다.

## 코드 품질 검사

Go 소스 변경 시 커밋 전에 아래 명령어를 실행한다.

```bash
cd mcp-servers/bridge

# 포맷 확인 (출력이 없으면 정상)
gofmt -l .

# 정적 분석
go vet ./...

# 종합 린트 (CI와 동일)
golangci-lint run --config .golangci.yml

# 테스트 (CI와 동일 timeout)
go test -count=1 -race -timeout 120s ./...
```

CI(`ci.yml`)가 PR마다 lint + test를 자동 실행한다.

### Pre-commit 훅 (lefthook)

```bash
go install github.com/evilmartians/lefthook@latest
lefthook install
```

`lefthook install` 이후 `git commit` 시 자동으로 gofmt, go vet, golangci-lint가 실행된다.

## 동작 확인

```bash
/mcp     # bridge · ✔ connected
head -3 ~/.claude/skills/oh-my-bridge/SKILL.md  # name: oh-my-bridge:code-routing
```
