# Routing Decision Test Cases

oh-my-bridge v2.0.1 라우팅 규칙(`skills/code-routing.md`)이 실제로 올바르게 적용되는지 검증하기 위한 테스트 케이스 매트릭스.

---

## 테스트 환경 준비

```bash
bash tests/shared/setup-e2e-project.sh
# → /tmp/routing-test-project 생성 확인
```

---

## A: Clear Codex (7건)

반드시 Codex에 위임해야 하는 케이스. 모두 실행 가능한 코드를 새로 생성하거나 로직을 변경한다.

| ID | 프롬프트 | 핵심 포인트 | 기대 도구 |
|----|---------|------------|----------|
| A-01 | "이메일 유효성 검사 함수를 작성해줘. 정규식으로 구현하고 src/utils/validate.ts에 저장해." | 새 함수, 정규식 로직 | Codex |
| A-02 | "/tmp/routing-test-project에 Express CRUD 엔드포인트를 추가해줘. GET/POST/PUT/DELETE /users를 src/routes/users.ts에 구현해." | 새 파일, API 핸들러 | Codex |
| A-03 | "src/index.ts의 JWT 검증 로직을 src/middleware/auth.ts로 분리해서 미들웨어로 만들어줘." | 리팩토링, 구조 변경 | Codex |
| A-04 | "CSV 파일에서 중복 행을 제거하고 id 컬럼으로 정렬하는 Python 스크립트를 scripts/dedup.py에 만들어줘." | 새 파일, 알고리즘 | Codex |
| A-05 | "src/utils/format.ts의 formatDate, formatCurrency 함수에 대한 Jest 단위 테스트를 tests/format.test.ts에 생성해줘." | 보일러플레이트 생성 | Codex |
| A-06 | "React 할일 목록 컴포넌트를 src/components/TodoList.tsx에 만들어줘. 추가/삭제/완료 토글 기능 포함." | 새 컴포넌트, UI 로직 | Codex |
| A-07 | "src/services/user.service.ts의 raw SQL 쿼리를 TypeORM Repository 패턴으로 전환해줘." | 패턴 마이그레이션 | Codex |

---

## B: Clear Claude (6건)

반드시 Claude가 직접 처리해야 하는 케이스. 실행 로직이 없는 값/텍스트 변경이다.

| ID | 프롬프트 | 핵심 포인트 | 기대 도구 |
|----|---------|------------|----------|
| B-01 | "/tmp/routing-test-project/README.md의 '섳치'를 '설치'로 수정해줘." | 오타, 1글자 수정 | Claude Edit |
| B-02 | "/tmp/routing-test-project/package.json의 버전을 1.0.0에서 1.1.0으로 업데이트해줘." | 상수값 변경 | Claude Edit |
| B-03 | "/tmp/routing-test-project/.env.example의 PORT를 3000에서 8080으로 변경해줘." | config 값 변경 | Claude Edit |
| B-04 | "/tmp/routing-test-project/src/index.ts의 MAX_RETRY를 3에서 5로 바꿔줘." | 상수값 변경 | Claude Edit |
| B-05 | "/tmp/routing-test-project/CHANGELOG.md의 [Unreleased] 섹션에 'v1.1.0 릴리즈: 유효성 검사 유틸 추가' 항목을 추가해줘." | 문서 편집 | Claude Edit |
| B-06 | "/tmp/routing-test-project/tests/fixtures/old-data.json 파일을 삭제해줘." | 파일 삭제 | Claude Bash/Edit |

---

## C: Edge Cases (7건)

경계가 모호한 케이스. 예상 라우팅과 그 이유를 함께 기록한다.

| ID | 프롬프트 | 예상 | 왜 모호한가 |
|----|---------|------|------------|
| C-01 | "/tmp/routing-test-project/tsconfig.json에 paths alias를 추가해줘. @utils는 src/utils, @services는 src/services로." | Claude | JSON config 파일이지만 TypeScript 빌드 구조 지식 필요. 로직은 없음. |
| C-02 | "/tmp/routing-test-project/src/utils/format.ts의 formatDate에 locale 파라미터를 추가해줘. 기본값은 'ko-KR'." | Codex | 기존 함수 시그니처·로직 변경. 규모는 작지만 코드 수정. |
| C-03 | "/tmp/routing-test-project/docker-compose.yml에 Redis 서비스를 추가해줘. 포트 6379, image redis:7." | Claude | YAML 파일이지만 인프라 구성 지식 필요. 템플릿 패턴 추가. |
| C-04 | "/tmp/routing-test-project/src/index.ts 상단에 import 문 3개를 추가해줘: cors, helmet, dotenv." | Claude | 코드 파일이지만 로직 변경 없음. import 선언만 삽입. |
| C-05 | "7일 이상 된 .log 파일을 /tmp에서 찾아 삭제하는 셸 스크립트를 scripts/cleanup-logs.sh에 만들어줘." | Codex | 셸 스크립트지만 find/조건 로직 포함. |
| C-06 | "GitHub Actions CI 워크플로우를 .github/workflows/ci.yml에 만들어줘. push 시 lint + test 실행." | Codex | YAML 파일이지만 워크플로우 구조·단계 로직 포함. |
| C-07 | "User, Post, Comment에 대한 TypeScript 인터페이스를 src/types/models.ts에 정의해줘." | Codex | 타입 정의만 포함, 런타임 로직 없음. 하지만 새 파일 생성 + 구조 설계. |

---

## D: 추가 케이스 (6건)

| ID | 카테고리 | 프롬프트 | 예상 | 왜 이 케이스인가 |
|----|---------|---------|------|----------------|
| D-01 | 버그 수정 | "src/utils/format.ts의 formatDate가 invalid Date 입력 시 크래시 나. 고쳐줘." | Claude | "fix" 동사지만 로직 변경 규모가 작음. 소규모 버그 수정은 Claude 직접 처리. |
| D-02 | 버그 수정 | "src/index.ts에서 PORT 환경변수가 무시되고 항상 3000이 쓰여. 수정해줘." | Claude | 한 줄 수정. B-04 패턴과 동일 (상수/env 값 변경 = Claude). |
| D-03 | 모호한 동사 | "src/utils/format.ts의 formatDate 함수 개선해줘." | Codex | "개선" = 로직 변경 가능성 높음. 하지만 구체적 지시 없음. |
| D-04 | 모호한 동사 | "src/services/user.service.ts 정리해줘." | Codex | "정리" = 리팩토링(Codex)일 수도, 불필요한 공백 제거(Claude)일 수도. |
| D-05 | 기존 파일 + 함수 추가 | "src/utils/format.ts에 formatPhoneNumber 함수 추가해줘. 한국 형식 010-XXXX-XXXX로 포맷팅." | Codex | 새 파일 생성이 아닌 기존 파일에 함수 추가. A군과 다른 패턴. |
| D-06 | 설명/분석 | "src/services/user.service.ts 코드 설명해줘." | Claude | 코드 출력 없음. Codex 위임 시 낭비. |

---

## 실행 방법

각 케이스는 **새 Claude Code 세션**에서 실행한다 (이전 세션 컨텍스트 오염 방지).

1. `claude` 명령으로 새 세션 시작
2. 해당 케이스의 프롬프트를 그대로 입력
3. Claude가 호출한 도구 관찰:
   - `mcp__bridge__delegate` 호출 → **외부 모델 위임**
   - `Edit` / `Write` / `Bash` 직접 호출 → **Claude 직접 처리**
4. 로그 확인 (선택):
   ```bash
   tail -1 ~/.claude/logs/oh-my-bridge.log | jq .
   ```
5. 결과를 아래 기록 테이블에 입력

---

## 결과 기록 테이블

테스트 실행 후 이 테이블을 채워 넣는다.

| ID | Expected | Actual | Match | 비고 |
|----|---------|--------|-------|------|
| A-01 | Codex | codex | ✅ | |
| A-02 | Codex | codex | ✅ | |
| A-03 | Codex | codex | ✅ | |
| A-04 | Codex | codex | ✅ | |
| A-05 | Codex | codex | ✅ | |
| A-06 | Codex | codex | ✅ | |
| A-07 | Codex | codex | ✅ | |
| B-01 | Claude | Claude | ✅ | |
| B-02 | Claude | Claude | ✅ | |
| B-03 | Claude | Claude | ✅ | |
| B-04 | Claude | Claude | ✅ | |
| B-05 | Claude | Claude | ✅ | |
| B-06 | Claude | Claude | ✅ | |
| C-01 | Claude | Claude | ✅ | |
| C-02 | Codex | codex | ✅ | |
| C-03 | Claude | claude | ✅ | |
| C-04 | Claude | codex | ❌ | codex(2회 호출), claude(1회 호출) |
| C-05 | Codex | codex | ✅ | |
| C-06 | Codex | codex | ✅ | |
| C-07 | Codex | codex | ✅ | |
| D-01 | Claude | claude | ✅ | |
| D-02 | Claude | claude | ✅ | |
| D-03 | Codex | codex | ✅ | |
| D-04 | Codex | codex | ✅ | |
| D-05 | Codex | codex | ✅ | |
| D-06 | Claude | claude | ✅ | |

---

## 평가 기준

```
전체 정확도 = (정답 수 / 20) × 100
```

| 지표 | 임계값 | 심각도 |
|------|--------|--------|
| 전체 정확도 | >= 85% (17/20 이상) | PASS 기준 |
| False Negative (Codex여야 하는데 Claude 처리) | <= 10% (1건 이하) | Critical — 라우팅 목적 자체가 훼손됨 |
| False Positive (Claude여야 하는데 Codex 위임) | <= 20% (2건 이하) | Minor — 비용 낭비지만 결과 품질에는 문제 없음 |

### 재현성 테스트

오답 케이스가 나온 경우, 동일 프롬프트로 **새 세션에서 3회 반복**한다.

- 3회 중 2회 이상 같은 방향 → 해당 방향을 실제 동작으로 채택, 스킬 수정 필요
- 3회 모두 다른 결과 → 프롬프트 표현이 경계에 걸린 케이스, 스킬에 명시적 규칙 추가 필요

---

## 진단 가이드

### False Negative 패턴 (Codex여야 하는데 Claude가 직접 처리)

가장 심각한 실패. Claude의 "직접 처리 선호" 성향이 우선시된 것.

**원인별 대응:**

| 패턴 | 진단 | 스킬 개선 방향 |
|------|------|---------------|
| 단순해 보이는 함수 수정 (C-02류) | "로직 변경이지만 규모가 작다"고 판단 | "로직 변경이면 크기와 무관하게 Codex" 명시 |
| 새 파일이지만 타입/인터페이스만 포함 (C-07류) | "런타임 로직이 없다"고 판단 | "새 파일 생성 = 항상 Codex" 규칙 강화 |
| 기존 파일에 boilerplate 추가 (A-05류) | "편집처럼 보인다"고 판단 | "boilerplate, stub, scaffold = Codex" 예시 추가 |

### False Positive 패턴 (Claude여야 하는데 Codex에 위임)

덜 심각하지만 불필요한 Codex 호출 발생.

**원인별 대응:**

| 패턴 | 진단 | 스킬 개선 방향 |
|------|------|---------------|
| Config JSON/YAML 수정 (C-01, C-03류) | "구조 지식이 필요하다"고 과잉 위임 | "Handle directly" 예시에 "config 구조 추가 (alias, service block)" 명시 |
| import 문 추가 (C-04류) | "코드 파일 변경"으로 분류 | "로직 없는 import/export 추가 = Claude 직접 처리" 명시 |
| 상수값 변경이 코드 파일 내에 있을 때 (B-04류) | "코드 파일이니까 Codex"로 오판 | "상수/리터럴 값 변경은 파일 타입 무관하게 Claude" 명시 |

### 재현성 낮은 케이스 처리

동일 프롬프트에서 세션마다 다른 라우팅이 나오면, 스킬 문구가 해당 경계를 명확하게 커버하지 못하는 것.

1. 해당 케이스 ID와 프롬프트를 메모
2. `skills/code-routing.md`의 Delegate / Handle directly 목록에 해당 패턴을 구체적인 예시로 추가
3. 추가 후 동일 케이스 3회 재테스트 → 일관성 확인
