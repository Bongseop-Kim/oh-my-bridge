package main

import "testing"

// TestDefaultTimeout_Codex verifies that codex uses the codex-specific default timeout.
func TestDefaultTimeout_Codex(t *testing.T) {
	got := defaultTimeout("codex")
	if got != defaultCodexTimeout {
		t.Errorf("defaultTimeout(codex) = %d, want %d", got, defaultCodexTimeout)
	}
}

// TestDefaultTimeout_Gemini verifies that gemini uses its own shorter default timeout.
func TestDefaultTimeout_Gemini(t *testing.T) {
	got := defaultTimeout("gemini")
	if got != defaultGeminiTimeout {
		t.Errorf("defaultTimeout(gemini) = %d, want %d", got, defaultGeminiTimeout)
	}
}

// TestDefaultTimeout_Unknown verifies that an unrecognised command falls back to
// the codex default rather than panicking or returning zero.
func TestDefaultTimeout_Unknown(t *testing.T) {
	got := defaultTimeout("unknown-cli")
	if got != defaultCodexTimeout {
		t.Errorf("defaultTimeout(unknown-cli) = %d, want codex fallback %d", got, defaultCodexTimeout)
	}
}

// TestDefaultTimeout_BelowMaxTimeout verifies that neither default exceeds maxTimeoutMs,
// which is the validation ceiling enforced in delegateTool.
func TestDefaultTimeout_BelowMaxTimeout(t *testing.T) {
	for _, cmd := range []string{"codex", "gemini"} {
		if dt := defaultTimeout(cmd); dt > maxTimeoutMs {
			t.Errorf("defaultTimeout(%q) = %d exceeds maxTimeoutMs %d", cmd, dt, maxTimeoutMs)
		}
	}
}
