package langchain

import (
	"testing"

	hippo "github.com/yunacaba/hippocampus"
)

func TestNewOllamaProvider_BuildsModel(t *testing.T) {
	p := NewOllamaProvider("")

	// Arbitrary, non-predefined model names are accepted verbatim.
	m, err := p.Model("gemma4", hippo.LLMType("gemma4"))
	if err != nil {
		t.Fatalf("Model: %v", err)
	}
	if m.Name() != "gemma4" {
		t.Errorf("name: want gemma4, got %q", m.Name())
	}
	if m.LLMVendor().String() != hippo.LLMVendorOllama.String() {
		t.Errorf("vendor: want ollama, got %q", m.LLMVendor().String())
	}
	// Ollama via langchaingo has no schema-enforcement mode; the agent falls
	// back to prompt guidance + the jsonx parser.
	if sc, ok := m.(interface{ SupportsResponseSchema() bool }); ok && sc.SupportsResponseSchema() {
		t.Error("Ollama model should not report response-schema support")
	}
}

func TestNewOllamaProvider_ServerURL(t *testing.T) {
	// Empty stays empty so langchaingo resolves OLLAMA_HOST at client init.
	if p := NewOllamaProvider(""); p.serverURL != "" {
		t.Errorf("empty server URL should not be defaulted, got %q", p.serverURL)
	}
	// An explicit URL is honored verbatim.
	custom := NewOllamaProvider("http://ollama.internal:11434")
	if custom.serverURL != "http://ollama.internal:11434" {
		t.Errorf("custom server URL not honored: %q", custom.serverURL)
	}
}

func TestProvider_RejectsEmptyType(t *testing.T) {
	for _, p := range []*Provider{NewOllamaProvider(""), NewProvider(nil)} {
		if _, err := p.Model("x", hippo.LLMType("")); err == nil {
			t.Error("want error for empty LLM type")
		}
	}
}
