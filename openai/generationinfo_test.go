package openai

import "testing"

// An empty or absent provider response must still return a writable
// GenerationInfo — see base.ModelCallResponse.GenerationInfo. Exercised by
// writing, because a nil map reads fine and would pass any inspection.
func TestResponseFromOpenAIAlwaysReturnsWritableGenerationInfo(t *testing.T) {
	resp := responseFromOpenAI(nil)
	if resp.GenerationInfo == nil {
		t.Fatal("nil completion: GenerationInfo is nil")
	}
	resp.GenerationInfo["k"] = "v"
}
