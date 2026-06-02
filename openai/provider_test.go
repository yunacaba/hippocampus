package openai_test

import (
	"testing"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/openai"
)

func TestProvider_AcceptsArbitraryModelName(t *testing.T) {
	p := openai.NewProvider(staticKeyProvider{key: "k"})
	// An unknown model string (e.g. a brand-new or OpenAI-compatible local model)
	// is accepted, using the string as the wire model id.
	if _, err := p.Model("m", hippo.LLMType("gpt-future-9")); err != nil {
		t.Errorf("expected arbitrary model name to be accepted, got %v", err)
	}
}

func TestProvider_RejectsKnownOtherVendor(t *testing.T) {
	p := openai.NewProvider(staticKeyProvider{key: "k"})
	// A model that is a known *other* vendor is rejected (catches misrouting).
	if _, err := p.Model("m", hippo.AnthropicClaudeHaiku45); err == nil {
		t.Error("expected a known Anthropic model to be rejected by the OpenAI provider")
	}
}
