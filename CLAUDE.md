# oh-my-bridge — CLAUDE.md

Claude가 코드 생성 작업을 자율 판단하여 적합한 AI 모델(Codex/Gemini)에 위임하는 skill 기반 브리지 플러그인.

## 작업 관점 기준 (CRITICAL)

이 repo의 작업은 두 관점이 공존한다. **문제를 받았을 때 반드시 먼저 관점을 판단하라.**

| 관점       | 언제                                            | 경로 기준                                                      |
| ---------- | ----------------------------------------------- | -------------------------------------------------------------- |
| **사용자** | MCP 연결 실패, 동작 문제, 설치 후 에러 | `install.sh` 재실행으로 해결 |
| **개발자** | 소스 코드 수정, 기능 추가, 버그 수정            | 이 repo 루트 (현재 작업 디렉토리)                              |

**규칙:**

- MCP 연결 실패 → **사용자 관점** → `install.sh` 재실행
- 소스 변경 후 배포 → **개발자 관점** → repo에서 빌드 후 `bump-version + install.sh 재실행`
- 절대로 사용자 문제를 개발자 repo 빌드로 해결하지 말 것

## 전제조건

```bash
npm install -g @openai/codex
npm install -g @google/gemini-cli
codex --version  # 설치 확인
gemini --version

# (선택) 린터 + pre-commit
brew install golangci-lint
go install github.com/evilmartians/lefthook@latest
lefthook install
```

## 핵심 명령어

```bash
# 설치 / 업데이트
curl -sSL https://raw.githubusercontent.com/Bongseop-Kim/oh-my-bridge/main/install.sh | bash

# 제거 (스킬+훅만, 바이너리 유지)
bash <(curl -sSL https://raw.githubusercontent.com/Bongseop-Kim/oh-my-bridge/main/uninstall.sh)

# 완전 제거 (바이너리+config 포함)
bash <(curl -sSL https://raw.githubusercontent.com/Bongseop-Kim/oh-my-bridge/main/uninstall.sh) --all

# 재배포 순서
# 1. ./bump-version.sh <new-version>  # 버전 업데이트 + commit + tag + push
# 2. git push origin <branch> → PR → main 머지
# 3. (2분 대기) GitHub Actions 릴리스 완료 대기
# 4. curl -sSL https://raw.githubusercontent.com/Bongseop-Kim/oh-my-bridge/main/install.sh | bash
# 5. Claude Code 재시작

# config 확인/편집
cat ~/.config/oh-my-bridge/config.json | jq .

# 사용 로그
tail -5 ~/.claude/logs/oh-my-bridge.log | jq .
```

## Skill 동작 방식

`install.sh` 실행 후 Claude는 `~/.claude/skills/oh-my-bridge/SKILL.md`를 세션마다 자동으로 읽는다.

- **코드 생성 작업** (새 파일, 함수/클래스 구현, 리팩토링) → `mcp__bridge__delegate` 호출 (`category` 필수, 바이너리가 `~/.config/oh-my-bridge/config.json`에서 모델 결정)
- **단순 편집** (오타, 한 줄 변경, config, 문서) → Claude 네이티브 Edit/Write 직접 사용
- **claude 응답** (`action: "claude"`) → Claude가 직접 처리 (route가 claude이거나 CLI 미설치)

MCP 호출 후에는 `Read` 도구로 생성 파일을 확인하고 결과를 보고한다.

## Gotchas

- **Skill은 세션 시작 시 로드** — 설치 후 Claude Code 재시작 필요
- **MCP 등록은 `install.sh`가 담당** — `claude mcp add --scope user`로 `~/.claude.json` 대신 user 스코프에 등록
- **config 없어도 MCP 기동 가능** — `loadConfig()`가 config 없으면 default config 자동 생성

## MCP Latency

MCP 툴(Codex, Gemini)은 Claude 네이티브 Write/Edit보다 3–4배 느리다. 단순 편집은 MCP를 거치지 않는 이유다.

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

CI(`ci.yml`)가 PR마다 lint + test를 자동 실행한다. `git commit` 시 pre-commit 훅(gofmt, go vet, golangci-lint)은 전제조건의 `lefthook install`로 활성화된다.

## 동작 확인

```bash
/mcp                                             # bridge · ✔ connected
head -3 ~/.claude/skills/oh-my-bridge/SKILL.md  # name: oh-my-bridge:code-routing
claude mcp list                                  # bridge 항목 확인
cat ~/.config/oh-my-bridge/config.json | jq .routes
```
