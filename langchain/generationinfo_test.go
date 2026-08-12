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

// The pass-through case, which is the one unique to this adapter: langchaingo
// leaves GenerationInfo nil whenever the underlying provider sets none, and
// forwarding that verbatim moves the panic to whichever caller next adds a key.
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
