# Claude Code 플러그인을 만들며 배운 것: 훅 강제 vs 규칙 유도

> oh-my-bridge 아키텍처를 전면 재설계하기까지의 고민 과정

---

## 시작: 단순한 아이디어

아이디어는 간단했다. Claude Code가 코드를 편집할 때, 그 편집을 OpenAI의 Codex CLI로 대신 실행하면 어떨까?

Claude는 오케스트레이터로 전체 흐름을 지휘하고, 실제 코드 생성은 GPT 계열 모델이 담당하는 구조다. Claude Code의 `PreToolUse` 훅을 활용하면 `Edit`와 `Write` 도구 호출을 가로채서 Codex로 라우팅하는 것이 기술적으로 가능했다.

이렇게 만들어진 것이 **oh-my-bridge**다.

---

## 훅 기반 인터셉터: 작동하지만 불편한 것

구현은 생각보다 간단했다.

```bash
# hooks/codex-interceptor.sh
# PreToolUse 훅: Edit|Write 호출을 가로채 Codex CLI로 라우팅
#
# Codex 성공 → permissionDecision: deny  (Codex가 파일 직접 수정 완료)
# Codex 실패 → permissionDecision: allow (Claude 네이티브 편집으로 폴백)
```

Claude가 `Edit`를 호출하면:

1. 훅이 가로채서 Codex CLI 실행
2. Codex가 파일을 직접 수정
3. 훅이 `permissionDecision: "deny"` 반환 → Claude의 네이티브 Edit 차단
4. 이중 편집 방지

기능은 완벽하게 동작했다. 그런데 Claude Code UI에서 이런 메시지가 보였다:

```
⏺ Update(src/auth.ts)
  ⎿  PreToolUse:Edit hook returned blocking error
  ⎿  Error: ✅ Codex가 파일을 수정했습니다...
```

**성공인데 Error로 표시된다.** `permissionDecision: "deny"`를 Claude Code UI가 에러로 렌더링하는 것이다. 기능은 정상이지만 사용자 경험이 나쁘다.

---

## 디버깅 중 발견한 버그, 그리고 비슷한 프로젝트

훅을 디버깅하던 중 다른 버그도 발견했다. 성공 응답을 출력하는 `jq` 명령 안에 싱글 쿼트가 포함된 메시지가 있었는데:

```bash
# 문제: 싱글 쿼트 문자열 안에 또 싱글 쿼트
jq -n '{
  permissionDecisionReason: "...'File has been modified since read' 에러가..."
}'
# → jq 파싱 에러 → 훅이 아무 출력도 내지 못하고 종료
```

쉘 스크립트의 고전적인 함정이었다. `--arg` 방식으로 해결했다.

버그를 고치면서 비슷한 프로젝트를 찾아봤다. **claude-delegator**라는 플러그인이 있었다. GitHub 스타 800+. 동일한 아이디어 — Claude Code 안에서 GPT를 활용하는 것 — 를 완전히 다른 방식으로 구현하고 있었다.

---

## claude-delegator가 문제를 피한 방법

claude-delegator의 구조:

```
rules/
  orchestration.md   ← "언제 누구에게 위임할지" 규칙
  triggers.md        ← 자동 감지 조건
  delegation-format.md
prompts/
  architect.md       ← 전문가별 시스템 프롬프트
  code-reviewer.md
  security-analyst.md
  ...
```

핵심은 **훅이 없다**는 것이다.

대신 `rules/` 파일들이 Claude Code 세션에 로드되어 Claude의 행동 기준이 된다. Claude는 코드 수정이 필요하다고 판단하면 `Edit`를 부르는 대신 **스스로** MCP를 호출한다:

```typescript
mcp__codex__codex({
  prompt: "...",
  "developer-instructions": "[expert system prompt]",
  sandbox: "workspace-write",  // Codex가 직접 파일 수정
  cwd: "/project"
})
```

흐름:
```
oh-my-bridge (훅):
  Claude → Edit 호출 → 훅 가로채기 → Codex 실행 → deny → ❌ Error 표시

claude-delegator (rules):
  Claude → rules 학습 → MCP 직접 호출 → Codex 실행 → ✅ 정상
```

`Edit`와 `Write`가 한 번도 호출되지 않으니 `deny`도, 에러도 없다.

---

## 두 방식의 본질적 차이

| | 훅 방식 (oh-my-bridge) | rules 방식 (claude-delegator) |
|---|---|---|
| **라우팅 기준** | 도구 이름 (Edit/Write) | 의도 (무엇을 하려는가) |
| **신뢰성** | 100% 보장 — 무조건 가로챔 | 비결정적 — Claude가 판단 |
| **UI 오류** | deny → Error 표시 | MCP 성공 → 오류 없음 |
| **커버리지** | 모든 Edit/Write | Claude가 선택한 경우만 |
| **제어권** | 플러그인이 강제 | Claude가 자발적으로 따름 |

---

## 근본 질문: 훅으로 강제하는 것이 맞는가?

이 시점에서 처음의 설계 가정을 다시 봤다.

> **"모든 코드 편집을 Codex로 라우팅한다"**

그런데 실제 `Edit` 호출의 성격은 균일하지 않다:

```
"TODO 주석 하나 추가"         → Edit → Codex 강제 실행  ← 낭비
"인증 모듈 전체 재작성"       → Edit → Codex 강제 실행  ← 합리적
"오타 하나 수정"              → Edit → Codex 강제 실행  ← 과잉
```

**코드 생성에 Codex가 유리하다는 근거는 맞지만, 모든 Edit가 코드 생성은 아니다.**

라우팅 기준이 "의도"가 아닌 "도구명"이라는 것이 문제였다.

---

## Claude와 GPT의 성격 차이

라우팅 근거를 다시 정리하면 단순한 성능 차이가 아니다.

| 모델 | 성격 | 프롬프트 스타일 |
|------|------|----------------|
| Claude | mechanics-driven | 상세한 체크리스트, 단계별 절차. "정확히 이 순서대로" |
| GPT | principle-driven | 간결한 원칙과 목표. "이 목적을 달성하라, 방법은 자율" |

실제로 복잡한 AI 에이전트 시스템에서 Prometheus 에이전트(계획 담당)의 Claude 프롬프트는 약 1,100줄인 반면, GPT 프롬프트는 약 121줄이다. 동일한 작업을 Claude에게는 세밀한 절차로, GPT에게는 간결한 원칙으로 지시한다.

이 성격 차이로부터 역할 배분이 도출된다:

- **오케스트레이터 → Claude**: 복잡한 멀티스텝 워크플로우를 정확히 따라야 하므로 mechanics-driven 특성이 유리
- **코드 생성 → GPT**: 구체적인 목표를 주고 자율 실행시키는 것이 효율적이므로 principle-driven 특성이 적합

여기서 모순이 드러난다. **Claude가 오케스트레이터 역할을 해야 한다면, Claude가 "이건 Codex한테 맡기자"고 스스로 판단하게 두는 것이 맞다.** 훅으로 강제하는 것은 오케스트레이터의 판단 권한을 빼앗는 것이다.

---

## 결정: skill 기반으로 전환

결론은 명확해졌다.

훅 방식은 **"도구 호출"을 라우팅 기준으로 삼는 구조적 한계**가 있다. 반면 skill 기반 방식은 **"의도"를 기준으로 Claude가 자율 판단**한다. 코드 생성에 GPT가 유리하다는 원칙과도 일관된다.

**새로운 구조:**

```
skills/
  code-routing.md   ← "복잡한 코드 생성/리팩토링은 MCP 호출, 단순 수정은 Claude 직접"
```

Claude가 코드 생성이 필요하다고 판단하면:
1. `Edit` 대신 `mcp__plugin_oh-my-bridge_codex__codex()` 호출
2. Codex가 `workspace-write` 샌드박스로 직접 파일 수정
3. Claude가 결과 합성
4. 단순 수정은 Claude가 직접 처리

비결정적이라는 단점이 있지만, claude-delegator의 사례에서 보듯 사용자들은 이 수준의 자율성을 수용한다.

---

## 라우팅 검증: 26개 테스트 케이스

skill 기반으로 전환한 뒤 한 가지 불안감이 남았다. **비결정적이라는 단점.** 훅은 무조건 가로채지만, 스킬은 Claude가 판단한다. 의도한 대로 라우팅되는지 실제로 확인해야 했다.

그래서 라우팅 결정 테스트 매트릭스를 만들었다 (`tests/routing-cases.md`).

**테스트 구성:**

| 카테고리 | 케이스 수 | 설명 |
|---------|---------|------|
| A: Clear Codex | 7건 | 반드시 Codex에 위임해야 하는 케이스 (새 파일, 로직 구현, 리팩토링) |
| B: Clear Claude | 6건 | 반드시 Claude가 직접 처리해야 하는 케이스 (오타, 값 변경, 문서 편집) |
| C: Edge Cases | 7건 | 경계가 모호한 케이스 (config 파일, import 추가, YAML 구성) |
| D: 추가 케이스 | 6건 | 버그 수정, 모호한 동사, 설명 요청 |

각 케이스는 새 Claude Code 세션에서 프롬프트를 그대로 입력하고, Claude가 `mcp__plugin_oh-my-bridge_codex__codex`를 호출하는지 아니면 `Edit`/`Write`를 직접 쓰는지 관찰했다.

**결과: 25/26 정답 (96.2%)**

유일한 오답은 C-04였다:

```
C-04: "import 문 3개를 추가해줘 (cors, helmet, dotenv)"
예상: Claude 직접 처리
실제: Codex 위임 (2회 호출)
```

import 문은 로직이 없어 Claude가 직접 처리해야 하는데 Codex로 라우팅됐다. False Positive — 결과 품질 문제는 없지만 불필요한 Codex 호출이 발생했다. 스킬에 "로직 없는 import/export 추가 = Claude 직접 처리" 규칙을 보강했다.

96%는 충분히 실용적인 수치다. 훅이 제공했던 100% 보장 대신, 약간의 비결정성을 받아들이는 대신 UX와 아키텍처 일관성을 얻었다.

---

## 다음 문제: Plan mode와의 통합

skill 기반으로 전환한 뒤 새로운 한계가 드러났다. Claude Code의 `EnterPlanMode`를 사용하면 — 즉 계획을 먼저 세우고 승인을 받은 뒤 실행하는 워크플로우에서 — oh-my-bridge가 전혀 작동하지 않는 것이다.

### 왜 Plan mode에서 code-routing이 트리거되지 않는가

스킬 description은 이렇게 되어 있다:

```
Use when you are about to write code, create new files, or implement features.
```

**"about to write code"** — 실행 직전 상태를 의미한다. Plan mode는 실행 이전 단계다.

```
[Plan mode]
사용자 요청 → 탐색/분석 → 계획 수립 → 승인 대기
                                          ↕ 여기서 멈춤
[Implementation mode]
승인 → 코드 생성  ← "about to write code" 조건이 충족되는 시점
```

Plan mode에서 Claude는 "코드를 작성 직전"이 아니라 "코드 작성 방법을 계획 중"이다. 스킬의 트리거 조건이 의미론적으로 false가 된다. 또한 Plan mode에서 MCP를 호출하는 것은 사용자가 계획을 승인하기 전에 코드가 생성된다는 뜻이므로, 설계 의도에도 맞지 않는다.

### ExitPlanMode 직후에도 트리거되지 않는 이유: 순환 의존성

문제는 Plan mode에서 끝나지 않는다. ExitPlanMode 이후에도 oh-my-bridge가 트리거되지 않는다.

이유는 순환 구조에 있다:

```
Plan mode에서 code-routing 미실행
        ↓
플랜이 Claude-native 도구로 기술됨
("src/auth.ts를 Edit 도구로 수정한다")
        ↓
ExitPlanMode → "플랜에 따라 실행"
        ↓
Claude-native로 구현
        ↓
oh-my-bridge 트리거 기회 없음
        ↓
(다음 플랜도 code-routing 없이 작성됨)
```

code-routing이 라우팅 결정을 해야 하는 시점(plan 작성)에 작동하지 않기 때문에, 플랜 자체가 "Claude가 한다"는 전제로 쓰인다. ExitPlanMode 이후 Claude는 이 플랜을 따르므로 라우팅 판단 기회가 없다.

ExitPlanMode가 보내는 시스템 메시지("You can now make edits, run tools, and take actions")는 강한 실행 신호다. Claude는 "실행해"라는 지시를 받고 플랜 단계를 직접 수행하기 시작한다. 스킬 체크가 들어갈 틈이 없다.

### 해결: routing pass

강제 호출은 oh-my-bridge의 설계 철학과 맞지 않는다. 이 스킬은 Claude가 모델 성격 차이를 이해하고 **스스로 판단**하도록 설계되어 있다. "무조건 Codex"가 아니라 "의도에 따라 Claude가 결정"이 핵심이다.

해결책은 **판단 기회를 만드는 것**이다. 강제가 아니라 라우팅 검토 단계를 삽입한다.

`CLAUDE.md`에 한 줄:

```
ExitPlanMode 이후 첫 번째 단계 실행 전에,
oh-my-bridge:code-routing 라우팅 규칙을 각 단계에 적용하여
Codex 위임 여부를 판단한다.
```

결과:

```
ExitPlanMode
    ↓
[라우팅 패스 — Claude의 판단]
  1단계: src/auth.ts JWT 구현 → 로직 있음 → Codex
  2단계: config.json 값 변경  → 로직 없음 → Claude 직접
  3단계: middleware 구현       → 로직 있음 → Codex
    ↓
oh-my-bridge 라우팅 규칙 적용, Codex 자연스럽게 호출
```

플랜 실행이 시작되기 전 Claude가 각 단계를 oh-my-bridge의 라우팅 기준으로 한 번 훑는다. Codex를 강제하는 것이 아니라, 판단이 일어날 공간을 만드는 것이다. 판단은 여전히 Claude가 한다.
