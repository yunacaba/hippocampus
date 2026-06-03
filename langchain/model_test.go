package langchain

import "testing"

func TestFirstGenInfoInt(t *testing.T) {
	// Google AI key.
	gi := map[string]any{"InputTokens": 12, "OutputTokens": 34}
	if got := firstGenInfoInt(gi, "InputTokens", "PromptTokens"); got != 12 {
		t.Errorf("Google input tokens: want 12, got %d", got)
	}

	// Ollama key (fallback when the primary key is absent).
	oi := map[string]any{"PromptTokens": 7, "CompletionTokens": 9}
	if got := firstGenInfoInt(oi, "InputTokens", "PromptTokens"); got != 7 {
		t.Errorf("Ollama prompt tokens: want 7, got %d", got)
	}
	if got := firstGenInfoInt(oi, "OutputTokens", "CompletionTokens"); got != 9 {
		t.Errorf("Ollama completion tokens: want 9, got %d", got)
	}

	// None present, or wrong type → 0.
	if got := firstGenInfoInt(map[string]any{"PromptTokens": "x"}, "InputTokens", "PromptTokens"); got != 0 {
		t.Errorf("missing/wrong-type: want 0, got %d", got)
	}
}
