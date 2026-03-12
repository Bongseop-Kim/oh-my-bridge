# 모델 유지보수 가이드

## 트리거

**외부 레포 참조:** `src/shared/model-requirements.ts`와 `CATEGORY_MODEL_REQUIREMENTS`는 이 레포에 없고 [oh-my-opencode -> oh-my-openagent](https://github.com/code-yeongyu/oh-my-openagent) 레포에 위치한다. oh-my-openagent의 해당 파일이 업데이트될 때 아래 파일들을 함께 수정한다.

## 변환 규칙

1. `claude-*` → **Claude (직접)** (oh-my-bridge 자체가 Claude)
2. OpenAI / Google → 공식 MCP (Codex CLI / Gemini CLI)

## 업데이트 파일 (3개 동시)

| 파일 | 섹션 |
|------|------|
| `skills/model-routing.md` | `## Fallback Chain` |
| `agents/codex-generator.md` | `### Step 2` |
| `README.md` | `## 카테고리별 Fallback Chain` |

## 현재 Fallback Chain (2026-03-11)

| 카테고리 | 1순위 | 2순위 | 3순위 | 4순위 |
|---------|------|------|------|------|
| `visual-engineering` | Gemini Pro (high) | Claude (직접) | — |
| `ultrabrain` | GPT-5.3 Codex (xhigh) | Gemini Pro (high) | Claude (직접) |
| `deep` | GPT-5.3 Codex (medium) | Claude (직접) | Gemini Pro (high) |
| `artistry` | Gemini Pro (high) | Claude (직접) | GPT-5.4 |
| `quick` | Claude (직접) | Gemini Flash | GPT-5-Nano |
| `writing` | Gemini Flash | Claude (직접) | — |
| `unspecified-high` | GPT-5.4 (high) | Claude (직접) | — |
| `unspecified-low` | Claude (직접) | GPT-5.3 Codex (medium) | Gemini Flash |

