# Config TUI 설계 문서

**날짜:** 2026-03-12
**버전:** 1.1
**범위:** `oh-my-bridge config` 서브커맨드 — 카테고리별 모델 할당 TUI

---

## 배경 및 목적

현재 모델 라우트 변경은 `~/.config/oh-my-bridge/config.json`을 직접 편집해야 한다.
JSON 직접 편집은 오타 위험이 있고, 유효한 모델 이름을 외워야 한다.

**목표:** 카테고리별 모델 할당을 오타 없이 변경할 수 있는 인터랙티브 TUI 제공.

---

## 범위

### 포함

- 8개 고정 카테고리의 모델 할당 변경 (드롭다운)
- 저장 전 정합성 검증 (route → model 존재 여부)
- `oh-my-bridge config list` 비대화형 출력 (스크립트/확인용)
- `oh-my-bridge config validate` 현재 config 검증 (exit code 포함)

### 제외

- 카테고리 추가/삭제 — `code-routing.md` 수정 필요, 개발자 영역
- 모델 정의(`models` 섹션) 추가/삭제 — 개발자 영역 (새 CLI 버전 출시 시 관리)

---

## 사용 흐름

```bash
oh-my-bridge config          # TUI 실행
oh-my-bridge config list     # 현재 라우트 테이블 출력
oh-my-bridge config validate # config 검증만 수행 (exit 0: 정상, exit 1: 오류)
```

### TUI 화면

```text
┌─ oh-my-bridge config ──────────────────────────┐
│                                                  │
│  Category             Model                      │
│  ────────────────     ─────────────────          │
│  visual-engineering   gemini-3-pro        [▼]    │
│  ultrabrain           gpt-5.3-codex       [▼]    │
│  deep                 gpt-5.3-codex       [▼]    │
│  artistry             gemini-3-pro        [▼]    │
│  quick                claude              [▼]    │
│  writing              gemini-3-flash      [▼]    │
│  unspecified-high     gpt-5.4             [▼]    │
│> unspecified-low      claude              [▼]    │  ← 선택된 행
│                                                  │
│  [↑↓] 이동  [Enter] 모델 변경  [s] 저장  [q] 종료   │
└──────────────────────────────────────────────────┘
```

### 드롭다운 (Enter 시)

```text
  unspecified-low 모델 선택:
  ────────────────────────
  ● claude              ← 현재 값 (고정 항목 — models 섹션과 무관)
    gpt-5.4
    gpt-5.3-codex
    gpt-5.3-codex-spark
    gemini-3-pro
    gemini-3-flash
    gemini-2.5-pro
    gemini-2.5-flash

  [↑↓] 이동  [Enter] 선택  [Esc] 취소
```

**드롭다운 목록 구성 규칙:**
- `"claude"` 는 항상 첫 번째에 고정 포함 (hardcoded sentinel — `models` 키가 아님)
- 나머지는 `config.json`의 `models` 키에서 동적으로 읽음
- → 오타 원천 차단

### 미저장 변경사항 종료 시

`[q]` 입력 시 미저장 변경사항이 있으면 확인 프롬프트 표시:

```text
  저장하지 않은 변경사항이 있습니다.
  [s] 저장 후 종료  [q] 버리고 종료  [Esc] 취소
```

변경사항이 없으면 바로 종료. 확인 프롬프트 내에서 다시 `[q]`를 누르면 "버리고 종료"와 동일하게 동작한다 (idempotent).

---

## 아키텍처

### 기술 스택

| 항목 | 선택 | 이유 |
|---|---|---|
| TUI 프레임워크 | [Bubble Tea](https://github.com/charmbracelet/bubbletea) | Go 생태계 표준, 별도 런타임 불필요 |
| 스타일링 | [Lip Gloss](https://github.com/charmbracelet/lipgloss) | Bubble Tea 공식 스타일 라이브러리 |

### 파일 변경 위치

```text
mcp-servers/bridge/
├── main.go          ← config 서브커맨드 진입점 추가 (dispatch 로직)
├── config_tui.go    ← TUI 모델 (Bubble Tea Model)
├── config_cmd.go    ← list / validate 비대화형 커맨드
├── go.mod           ← bubbletea, lipgloss 의존성 추가
└── go.sum
```

### main() 진입점 구조

현재 `main()`은 무조건 MCP stdio 서버로 기동된다. `config` 서브커맨드를 추가하기 위해 `loadConfig()` 호출 전에 args를 검사하여 분기한다:

```go
func main() {
    // 1. config 서브커맨드 분기 — MCP 서버 기동 전에 처리
    if len(os.Args) > 1 && os.Args[1] == "config" {
        // loadConfig() 실패는 fatal이 아닌 에러 출력 후 exit 1
        if err := loadConfig(); err != nil {
            fmt.Fprintf(os.Stderr, "config load error: %v\n", err)
            os.Exit(1)
        }
        runConfigCommand(os.Args[2:]) // list / validate / TUI
        return
    }

    // 2. MCP 서버 모드 (기존 동작 — 변경 없음)
    if err := loadConfig(); err != nil {
        log.Fatalf("failed to load config: %v", err)
    }
    detectCLIs()
    // ... server.Run(...)
}
```

**Bubble Tea와 MCP stdio 충돌 방지:** `tea.NewProgram`은 `config` 분기 안에서만 호출되므로 MCP 서버 모드에서는 절대 실행되지 않는다. 두 모드가 동시에 활성화될 수 있는 코드 경로는 존재하지 않는다.

### 데이터 흐름

```text
main()
  → args[1] == "config"
      → loadConfig() (실패 시 exit 1, non-fatal)
      → runConfigCommand(args[2:])
          → "list"     → printConfigTable() → stdout
          → "validate" → validateConfig() → exit 0/1
          → (없음)     → tea.NewProgram(configModel) → TUI
  → (없음)
      → loadConfig() (실패 시 log.Fatalf)
      → detectCLIs()
      → server.Run(...)  ← MCP 서버 모드
```

### 저장 방식

변경사항은 메모리에 보관하다가 `[s]` 입력 시:

1. 전체 config 검증 (route → model 존재 여부)
2. 검증 실패 시 저장 차단 + TUI 내 에러 메시지 표시
3. 검증 통과 시:
   - `~/.config/oh-my-bridge/config.json.tmp` 에 쓰기 (같은 디렉토리 — atomic rename 보장)
   - `os.Rename("config.json.tmp", "config.json")` → 동일 파일시스템 내 atomic 교체

---

## 검증 규칙

| 규칙 | 설명 |
|---|---|
| route → model 존재 | 각 route 값이 `models` 키에 존재하거나 `"claude"` 이어야 함 |
| 필수 필드 | `routes`, `models` 섹션 모두 존재해야 함 |
| 8개 카테고리 존재 | 기본 카테고리 누락 시 경고 (오류는 아님) |

## `validate` 종료 코드

| 상황 | exit code |
|---|---|
| config 정상 | 0 |
| config 오류 (필드 누락, 잘못된 route 값 등) | 1 |
| config 파일 없음 / 읽기 실패 | 1 |

## `list` 출력 형식

사람이 읽기 쉬운 plain-text 테이블. 각 카테고리의 현재 모델과 해당 CLI 설치 여부를 함께 표시:

```text
Category             Model              CLI
────────────────     ──────────────     ───
visual-engineering   gemini-3-pro       gemini ✔
ultrabrain           gpt-5.3-codex      codex ✔
deep                 gpt-5.3-codex      codex ✔
artistry             gemini-3-pro       gemini ✔
quick                claude             —
writing              gemini-3-flash     gemini ✔
unspecified-high     gpt-5.4            codex ✔
unspecified-low      claude             —
```

- `claude` 라우트 행은 외부 바이너리가 불필요하므로 CLI 열에 `—` 표시
- `routes` 또는 `models` 섹션이 nil(비어 있음)인 경우: 빈 테이블을 출력하고 stderr에 에러 메시지 출력 후 exit 1

스크립트에서 파싱이 필요한 경우 기존 `mcp__bridge__status` MCP 도구의 JSON 출력을 사용한다.

---

## 배포

기존 `bump-version.sh` + GitHub Actions 그대로 사용.
바이너리 한 개에 서브커맨드만 추가되므로 설치 흐름 변경 없음.
사용자는 `/plugin update oh-my-bridge` + Claude Code 재시작으로 업데이트.

---

## 비범위 결정 근거

- **카테고리 추가 제외**: 카테고리는 `skills/code-routing.md`에 정의되어 있어, config만 수정하면 Claude가 새 카테고리를 인식하지 못함. 일관성 유지를 위해 개발자 영역으로 분리.
- **모델 정의 편집 제외**: `models` 섹션은 CLI 버전 업 시 패키지 개발자가 관리하는 영역. 사용자가 건드릴 일이 없으므로 TUI 복잡도를 낮추기 위해 제외.
