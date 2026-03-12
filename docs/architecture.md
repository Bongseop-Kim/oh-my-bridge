# oh-my-bridge — Architecture & Design Rationale

설계 결정의 이유를 기록한 문서. "왜 이렇게 만들었는가"에 답한다.

---

## 1. 왜 Claude가 오케스트레이터인가

능력의 차이가 아니라 **모델 성격(personality)의 차이**다.

| 모델 | 성격 | 최적 프롬프트 스타일 |
|------|------|-------------------|
| Claude | Mechanics-driven | 상세한 체크리스트, 단계별 절차, "정확히 이렇게 해" |
| Codex (GPT) | Principle-driven | 간결한 목표, 자율 실행, "이걸 달성해, 방법은 네가 결정" |
| Gemini Pro | Vision-driven | UI/UX 맥락, 시각적 목표, 전체 화면 구조 |

코드 생성은 원칙 중심 모델이 유리하다. Claude는 사용자 의도를 정밀한 프롬프트로 변환하고, 결과를 검증하는 역할에 집중한다. **Claude orchestrates — external models generate.**

---

## 2. 단일 Skill (`code-routing`) 구조

위임 여부 판단과 모델 선택이 `code-routing.md` 하나로 통합되어 있다.

| 섹션 | 관심사 | 질문 |
|------|--------|------|
| Routing rule | 위임 여부 | "결과가 실행 가능한 코드인가?" |
| Model Routing | 모델 선택 | "어떤 카테고리인가? → Fallback Chain 실행" |

두 관심사가 하나의 Skill 안에 있지만, 섹션으로 분리되어 있어 각각 독립적으로 수정할 수 있다.

---

## 3. MCP 서버 프로세스 생명주기

### Claude Code의 공식 동작

Claude Code는 MCP 서버를 **세션 시작 시 한 번만 spawn**하고 세션 내내 프로세스를 유지한다. 툴 호출마다 새 프로세스를 만들지 않는다.

각 툴 호출은 동일 프로세스에 **JSON-RPC 2.0 메시지**로 전달된다.

```
Claude Code ──(persistent stdio)──► MCP bridge server (Go binary)
                                          │
                                          └──(per-call exec)──► codex CLI
                                          └──(per-call exec)──► gemini CLI
```

### oh-my-bridge의 지연 원인

지연은 **MCP 서버 프로세스 재시작이 아니라**, bridge 서버 내부에서 Codex/Gemini CLI를 매번 `exec`으로 실행하고 LLM API 왕복이 추가되기 때문이다.

| 단계 | 소요 시간 | 설명 |
|------|---------|------|
| Go 바이너리 cold start | ~3ms | 세션 시작 시 1회 |
| Codex/Gemini CLI exec | ~5s | 매 툴 호출마다 |
| LLM API 왕복 (file write 기준) | +15–20s | 매 툴 호출마다 |
| **합계 (파일 생성 기준)** | **20–30s** | |
| Claude 네이티브 Write/Edit | ~7s | MCP 없음 |

단순 편집에 MCP를 거치지 않는 이유가 여기 있다.

---

## 4. Go 바이너리를 선택한 이유

플러그인으로 배포되는 특성상 사용자 환경에 Node.js나 npm이 설치되어 있다고 보장할 수 없다. Go 정적 바이너리는 런타임 없이 단일 파일로 실행되므로 GitHub Releases로 플랫폼별 바이너리를 배포하는 구조와 맞았다.

| 런타임 | cold start | 비고 |
|--------|-----------|------|
| Node.js | ~800ms | npm 의존성 포함 시 더 느림 |
| Go 정적 바이너리 | ~3ms | 의존성 없음, 단일 파일 배포 |

cold start 차이(800ms vs 3ms)는 세션 시작 시 1회 발생하므로 체감 차이는 미미하다. 주 이유는 런타임 의존성 제거다.

---

## 5. 참고

- [MCP Architecture — Model Context Protocol](https://modelcontextprotocol.io/docs/learn/architecture)
- [MCP Lifecycle Specification](https://modelcontextprotocol.io/specification/latest/basic/lifecycle)
- [Claude Code MCP docs](https://code.claude.com/docs/en/mcp.md)
