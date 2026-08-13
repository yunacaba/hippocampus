package langchain

import (
	"testing"

	"github.com/tmc/langchaingo/llms"
)

// An empty or absent provider response must still return a writable
// GenerationInfo — see base.ModelCallResponse.GenerationInfo.
func TestResponseFromLangchainAlwaysReturnsWritableGenerationInfo(t *testing.T) {
	resp := responseFromLangchain(nil)
	if resp.GenerationInfo == nil {
		t.Fatal("nil completion: GenerationInfo is nil")
	}
	resp.GenerationInfo["k"] = "v"
}

// The pass-through case, unique to this adapter: langchaingo hands back the
// provider's own map, so the adapter normalises what it is given rather than
// what it built.
//
// Defensive, not a live fix. Neither provider this package can construct omits
// GenerationInfo — ollama builds a populated literal and googleai a make() —
// and the langchaingo model is an unexported field reachable only through
// NewProvider and NewOllamaProvider, so no caller can supply a third. The
// guard earns its keep because this converter is provider-agnostic and cannot
// check which one produced the response, so a langchaingo bump or a new
// provider makes it live, at the cost of one function call. The choice hangs
// on that, not on the case being reachable today.
//
// This fixture is therefore a shape no wired provider currently emits, which
// is the point: it pins the normalisation rather than the provider.
func TestResponseFromLangchainFillsAnAbsentGenerationInfo(t *testing.T) {
	resp := responseFromLangchain(&llms.ContentResponse{
		Choices: []*llms.ContentChoice{{Content: "hi"}}, // GenerationInfo nil
	})
	if resp.GenerationInfo == nil {
		t.Fatal("provider set no GenerationInfo and the adapter passed the nil through")
	}
	resp.GenerationInfo["k"] = "v"

	if resp.Content != "hi" {
		t.Errorf("Content = %q, want hi", resp.Content)
	}
}
