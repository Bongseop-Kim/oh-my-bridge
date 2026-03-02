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

## 배운 것

1. **훅(hook)은 강력하지만 맹목적이다.** 도구 이름만 보고 의도를 모른다.

2. **skill 기반 유도가 LLM 에이전트에 더 자연스럽다.** LLM은 규칙을 "따르도록" 설계되어 있다. 외부에서 강제하는 것보다 내부 판단으로 유도하는 것이 아키텍처적으로 일관된다.

3. **비슷한 아이디어가 이미 있다는 것은 방향이 맞다는 신호다.** 800+ 스타는 시장 검증이기도 하다. 구현 방식의 차별화를 고민하는 게 더 생산적이다.

4. **UX 문제는 종종 아키텍처 문제의 신호다.** deny-as-error는 단순한 메시지 문제가 아니라 "강제 가로채기"라는 구조적 선택의 결과였다.

---

## 다음

oh-my-bridge를 skill 기반으로 전환한다. 훅을 제거하고, Claude가 코드 생성 작업을 스스로 Codex에 위임하도록 스킬을 작성한다.

"모든 편집을 Codex로" 에서 "코드 생성을 Codex에게" 로.
