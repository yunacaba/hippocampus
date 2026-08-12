package anthropic

import "testing"

// An empty or absent provider response must still return a writable
// GenerationInfo — see base.ModelCallResponse.GenerationInfo. This is the path
// that panicked Generate when request-id reporting began writing into it.
func TestResponseFromAnthropicAlwaysReturnsWritableGenerationInfo(t *testing.T) {
	resp := responseFromAnthropic(nil)
	if resp.GenerationInfo == nil {
		t.Fatal("nil message: GenerationInfo is nil")
	}
	resp.GenerationInfo["k"] = "v"
}
