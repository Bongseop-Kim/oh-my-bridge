# Issue #10 — Codex CLI 무응답 상태 조기 탐지 불가

## 문제

Codex CLI 모델 불일치(예: config `gpt-5.4` ↔ 호출 `gpt-5.3-codex`) 발생 시
CLI가 응답 없이 hang → `runCli`가 전체 `timeoutMs`(기본 180s)를 소진 후 실패.

## 단위 테스트 실행

```bash
cd mcp-servers/bridge

# 전체 실행
go test -v -run "TestRunCli_|TestRunCodex_" -timeout 30s

# 개별 실행
go test -v -run TestRunCli_HangUntilTimeout      # 버그 재현 (2s 대기)
go test -v -run TestRunCli_NoFirstTokenDetection  # 구조 확인 (즉시)
go test -v -run TestRunCodex_HangSimulation       # end-to-end 재현 (2s 대기)
go test -v -run TestRunCli_ModelMismatch_FastExit # 빠른 exit 확인 (즉시)

# hang simulation 제외하고 빠르게
go test -short -v -run "TestRunCli_|TestRunCodex_"
```

## 예상 출력 (fix 전)

```
--- PASS: TestRunCli_HangUntilTimeout (2.00s)
    BUG REPRODUCED: waited full 2000ms. With heartbeat fix, should fail within 500ms.

--- PASS: TestRunCli_NoFirstTokenDetection (0.00s)
    cliRequest has no HeartbeatTimeoutMs — heartbeat detection not implemented (issue #10 open)

--- PASS: TestRunCodex_HangSimulation (2.00s)
    BUG REPRODUCED (issue #10): runCodex waited ~2s. Fix: heartbeat should abort within 30s.

--- PASS: TestRunCli_ModelMismatch_FastExit (0.00s)
    fast-exit returned error immediately (desired behavior)
```

## Fix 검증 방법

수정 후 아래 두 가지를 확인한다.

1. `cliRequest`에 `HeartbeatTimeoutMs` 필드 추가됐는지 컴파일로 확인:
   ```go
   // issue10_hang_test.go 내 주석 해제
   _ = req.HeartbeatTimeoutMs
   ```

2. `TestRunCli_HangUntilTimeout` elapsed가 `heartbeatThresholdMs`(500ms) 이내인지 확인:
   ```
   elapsed: 487ms (fullTimeout: 2000ms, heartbeatThreshold: 500ms)
   ```

## 관련 파일

| 파일 | 설명 |
|------|------|
| `mcp-servers/bridge/issue10_hang_test.go` | 이슈 재현 단위 테스트 |
| `mcp-servers/bridge/testutil_test.go` | 공통 테스트 헬퍼 |
| `mcp-servers/bridge/main.go` | `runCli`, `runCodex` 구현 |
