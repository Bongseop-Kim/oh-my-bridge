# oh-my-bridge — Architecture & Design Rationale

설계 결정의 이유를 기록한 문서. "왜 이렇게 만들었는가"에 답한다.

---

## 1. MCP 서버 프로세스 생명주기

### Claude Code의 공식 동작

Claude Code는 MCP 서버를 **세션 시작 시 한 번만 spawn**하고 세션 내내 프로세스를 유지한다. 툴 호출마다 새 프로세스를 만들지 않는다.

각 툴 호출은 동일 프로세스에 **JSON-RPC 2.0 메시지**로 전달된다.

```text
Claude Code --(persistent stdio)--> MCP bridge server (Go binary)
                                          |
                                          +--(per-call exec)--> codex CLI
                                          +--(per-call exec)--> gemini CLI
```

### oh-my-bridge의 지연 원인

지연은 **MCP 서버 프로세스 재시작이 아니라**, bridge 서버 내부에서 Codex/Gemini CLI를 매번 `exec`으로 실행하고 LLM API 왕복이 추가되기 때문이다.

| 단계                           | 소요 시간  | 설명             |
| ------------------------------ | ---------- | ---------------- |
| Go 바이너리 cold start         | ~3ms       | 세션 시작 시 1회 |
| Codex/Gemini CLI exec          | ~5s        | 매 툴 호출마다   |
| LLM API 왕복 (file write 기준) | +15–20s    | 매 툴 호출마다   |
| **합계 (파일 생성 기준)**      | **20–30s** |                  |
| Claude 네이티브 Write/Edit     | ~7s        | MCP 없음         |

단순 편집에 MCP를 거치지 않는 이유가 여기 있다.

### CLI 실행 전략: Polling + Stability

CLI 프로세스는 자연 종료를 보장하지 않는다. Codex/Gemini는 작업을 마친 뒤에도 프롬프트 대기 상태로 멈출 수 있다. `runCli`는 두 전략으로 이를 처리한다.

**Polling** — `time.NewTicker(stabilityPollIntervalMs)` 로 주기적으로 `activityTracker`의 마지막 출력 시각을 확인한다. Codex의 `-o` 출력 파일 mtime도 함께 폴링해 파일 쓰기 활동을 감지한다.

**Stability timeout** — 마지막 활동으로부터 `StabilityTimeoutMs`이 경과하면 "출력이 안정됐다 = 작업 완료"로 판단하고 프로세스를 강제 종료, `StabilityExit: true` 를 반환한다.

타임아웃은 3단계로 구성된다:

| 단계         | 설정 키                | 기본값     | 조건                        | 결과                         |
| ------------ | ---------------------- | ---------- | --------------------------- | ---------------------------- |
| First-output | `FirstOutputTimeoutMs` | 30s        | 시작 후 첫 출력이 없는 경우 | `ErrTimeout` (에러)          |
| Stability    | `StabilityTimeoutMs`   | 10s        | 출력 후 조용해진 경우       | `StabilityExit: true` (정상) |
| Max ceiling  | `MaxTimeoutMs`         | 30min      | 절대 상한 초과              | `ErrTimeout` (에러)          |

폴링 간격: `stabilityPollIntervalMs` = 1s (고정값, config 불가). stability 감지의 정밀도는 ±1s인데, `StabilityTimeoutMs` 기본값이 10s이므로 오차가 행동에 영향을 주지 않는다. 폴링 간격은 사용자가 조정할 필요가 없는 내부 구현 세부사항이므로 config에 노출하지 않았다.

`StabilityExit: true` 반환 시 출력이 불완전할 수 있으므로 MCP Content 앞에 경고 prefix를 자동 삽입한다 (→ [섹션 5](#5-fallback-전략) 참조).

---

## 2. Skill 구조 — full vs slim

### 두 개의 Skill 파일

| 파일                     | 설치 위치                                            | 로드 시점                          |
| ------------------------ | ---------------------------------------------------- | ---------------------------------- |
| `code-routing-full.md`   | `~/.claude/skills/oh-my-bridge/SKILL.md`             | 메인 세션 시작 시 자동 로드        |
| `code-routing-slim.md`   | `~/.claude/skills/oh-my-bridge/code-routing-slim.md` | SubagentStart hook이 서브에이전트에 주입 |

### full이 메인 세션에 필요한 이유

위임 여부 판단과 모델 선택이 `SKILL.md` 하나로 통합되어 있다.

| 섹션          | 관심사    | 질문                                                    |
| ------------- | --------- | ------------------------------------------------------- |
| Routing rule  | 위임 여부 | "결과가 실행 가능한 코드인가?"                          |
| Model Routing | 모델 선택 | "어떤 카테고리인가? → config 라우트에서 단일 모델 결정" |

두 관심사가 하나의 Skill 안에 있지만, 섹션으로 분리되어 있어 각각 독립적으로 수정할 수 있다.

### slim이 서브에이전트에 필요한 이유

Claude가 서브에이전트를 띄우면 그 서브에이전트는 새 컨텍스트로 시작하므로 메인 세션에 로드된 `SKILL.md`를 모른다. SubagentStart hook이 서브에이전트 시작 시 slim을 주입해 "로직 변경이면 delegate해라"는 최소한의 룰을 전달한다.

full 전체를 주입하지 않는 이유: 서브에이전트는 좁은 단일 작업을 수행하므로 모델 성격 설명이나 카테고리 분류 근거는 불필요하고, 토큰 낭비다.

### full vs slim 내용 차이

slim은 full에서 다음 섹션을 제거한 버전이다:

| 섹션                         | full | slim | 제거 이유                                          |
| ---------------------------- | :--: | :--: | -------------------------------------------------- |
| Why multi-model routing (모델 성격 비교표) | ✅ | ❌ | 서브에이전트에게 "왜"는 불필요 — "무엇을"만 전달 |
| Before executing each plan step | ✅ | ❌ | 플랜 실행 흐름은 메인 세션 관심사                |
| Category 선택 근거/예시       | ✅ | ❌ | 카테고리 목록만으로 충분                           |
| 7-section 프롬프트 상세 예시  | ✅ | ❌ | 형식 이름만 전달, 세부는 메인 세션에서 구성       |
| CONTEXT section 작성 지침     | ✅ | ❌ | 파일 경로 vs 인라인 판단은 메인 세션 역할         |
| After delegation 상세 오류 처리 | ✅ | ❌ | MCP 실패 분기 처리는 메인 세션에서 수행           |
| Security 섹션                | ✅ | ❌ | 서브에이전트가 직접 판단할 필요 없음              |

slim을 수정할 때는 routing 판단 기준(`위임 여부`)과 카테고리 목록만 유지하고, 나머지는 full에서 관리한다.

---

## 3. Go 바이너리를 선택한 이유

**핵심 이유는 런타임 의존성 제거다.** `curl | bash` 원라인 설치로 배포되는 특성상 사용자 환경에 Node.js나 npm이 설치되어 있다고 보장할 수 없다. Go 정적 바이너리는 런타임 없이 단일 파일로 실행되므로 GitHub Releases로 플랫폼별 바이너리를 배포하는 구조와 맞았다.

| 런타임           | cold start | 비고                        |
| ---------------- | ---------- | --------------------------- |
| Node.js          | ~800ms     | npm 의존성 포함 시 더 느림  |
| Go 정적 바이너리 | ~3ms       | 의존성 없음, 단일 파일 배포 |

cold start 차이(800ms vs 3ms)는 세션 시작 시 1회만 발생하므로 체감 차이는 미미하다. 이 수치는 참고용이며, 선택의 이유가 아니다.

---

## 4. CLI vs MCP 설계 검토 (2026.03)

현재 bridge는 외부 모델을 **CLI로 호출**한다. MCP로 전환하는 방안을 검토했고, 그 결과를 기록한다.

### 현재 구조

```text
Claude Code
  └── bridge MCP (Go 바이너리)
        └── codex --full-auto  (CLI exec)
        └── gemini --yolo      (CLI exec)
```

### MCP 전환 시 구조

Codex는 `codex-mcp-server`를 공식 제공한다. Gemini는 커뮤니티 MCP 서버가 있으며 내부적으로 CLI를 subprocess로 호출한다.

```text
Claude Code
  └── bridge MCP (Go 바이너리)
        └── codex-mcp-server  (JSON-RPC)
        └── gemini-mcp-server (JSON-RPC → CLI subprocess)
```

### 항목별 비교

TES(Tool Execution Score): 코딩 에이전트 벤치마크에서 툴 실행 성공률을 수치화한 지표. 높을수록 우수. [출처: Zechner 2025.08]

| 항목             | CLI                            | MCP                                | 우위                     |
| ---------------- | ------------------------------ | ---------------------------------- | ------------------------ |
| 장시간 작업 제어 | 블로킹, kill만 가능            | 스트리밍, cancel, 부분 결과 수신   | MCP                      |
| 컨텍스트 전달    | 문자열 한 방                   | 구조화된 스키마                    | MCP (단, 토큰 비용 증가) |
| 에러 처리        | exit code + stderr 파싱        | JSON-RPC 타입별 에러 코드          | MCP                      |
| 멀티턴           | 불가 (매 호출 새 프로세스)     | threadId로 세션 연속               | MCP                      |
| 권한 제어        | 플래그 한 개                   | approval_policy, sandbox 세밀 제어 | MCP                      |
| CLI 변경 추적    | 봉섭님이 직접 추적             | 모델사가 MCP 서버 유지             | MCP                      |
| 토큰 효율        | TES 202 (벤치마크)             | TES 152, 최대 236x 증가 위험       | CLI                      |
| 보안             | 단순, injection 방어 직접 구현 | 공격 성공률 23~41% 증폭 위험       | CLI                      |
| 설정·디버깅      | 터미널에서 즉시 재현 가능      | JSON-RPC 페이로드 디버깅 필요      | CLI                      |

### 결론

**MCP 전환의 핵심 이점은 두 가지다.** 모델사가 CLI 변경을 흡수하므로 bridge 유지보수 부담이 줄고, threadId 기반 멀티턴 위임이 가능해진다.

**반면 CLI가 우위인 항목도 명확하다.** 토큰 효율(33% 우위), 보안(MCP 아키텍처 자체의 공격 증폭 특성), 디버깅 단순성.

현재 oh-my-bridge의 사용 패턴(단발성 코드 생성 위임)에서는 멀티턴 이점이 크지 않다. 다음 기준 중 하나를 충족하면 전환을 재검토한다:

- bridge에서 CLI 변경 대응 커밋이 **분기당 3회 이상** 발생
- 단일 위임 작업이 **멀티턴 컨텍스트를 필요**로 하는 사례가 반복

---

## 5. Fallback 전략

CLI 실행 오류가 발생해도 요청 전체가 실패하지 않도록 여러 레이어의 fallback을 구현했다.

### CLI 실행 오류 → Claude fallback

`classifyCliError` 헬퍼가 런타임 오류를 3가지로 분류한다:

| `reason` 값            | 조건                                   |
| ---------------------- | -------------------------------------- |
| `cli_error_timeout`    | 실행 시간 초과                         |
| `cli_error_rate_limit` | stderr 패턴 매칭으로 rate limit 감지   |
| `cli_error_crash`      | 그 외 비정상 종료 (exit code != 0)     |

이 경우 에러 대신 `action: "claude"`, `reason: "<분류>"` 를 반환한다. 단, `ErrUnsupportedCommand` (잘못된 config 등 설정 오류)는 여전히 hard error를 반환한다.

### reloadState 실패 → stale state 유지

config reload 실패 시 기존에 성공한 config/clis 상태를 유지한 채 요청을 계속 처리한다. 최초 로드 실패 또는 완전히 빈 상태에서는 여전히 에러를 반환한다.

### reason 필드 구분

로그의 `reason` 필드는 상수로 정의되어 다음 두 경우를 구분한다:

| `reason` 값         | 의미                                                         |
| ------------------- | ------------------------------------------------------------ |
| `route_configured`  | config에 `"route": "claude"` 로 명시된 경우 (의도적 라우팅) |
| `cli_not_installed` | CLI 바이너리가 설치되지 않은 경우 (환경 문제)               |

### StabilityExit 경고 prefix

`StabilityExit: true` 일 때 MCP Content 앞에 경고를 자동 삽입한다:

```text
[WARNING: output may be incomplete due to stability exit] ...
```

Skill이 파일 확인을 권고하는 것 외에, 사용자가 출력이 불완전할 수 있음을 즉시 인지할 수 있다.

### default_route — 라우팅 시점 fallback

위의 fallback들이 **런타임 오류**(CLI 크래시, 타임아웃 등) 발생 후 작동하는 것과 달리, `default_route`는 **라우팅 시점**에 작동한다. config의 `routes`에 없는 category 요청이 들어오면 CLI를 실행하기 전에 이 값으로 대신 라우팅한다.

```json
{
  "default_route": "unspecified-low"
}
```

미지정 시 기존과 동일하게 hard error를 반환한다.

---

## 6. 참고

- [MCP Architecture — Model Context Protocol](https://modelcontextprotocol.io/docs/learn/architecture)
- [MCP Lifecycle Specification](https://modelcontextprotocol.io/specification/latest/basic/lifecycle)
- [Claude Code MCP docs](https://code.claude.com/docs/en/mcp.md)
- Mario Zechner, [MCP vs CLI: Benchmarking Tools for Coding Agents](https://mariozechner.at/posts/2025-08-15-mcp-vs-cli/) (2025.08) — 120회 실험, 성공률 동일, 토큰 효율 CLI 33% 우위
- arXiv 2508.12566 — MCP 컨텍스트 통합 시 입력 토큰 최대 236.5x 증가
- arXiv 2601.17549 — MCP 아키텍처가 공격 성공률 23~41% 증폭
- arXiv 2602.14878 — MCP tool description 품질이 성공률에 직접 영향
- [Codex MCP Server 공식 문서](https://developers.openai.com/codex/guides/agents-sdk/)
