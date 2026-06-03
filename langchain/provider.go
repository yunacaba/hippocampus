package langchain

import (
	"context"
	"fmt"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/base"
)

// OllamaServerURL is the default base URL for a local Ollama server's native
// API. Note this is the native endpoint (no /v1 suffix), unlike the
// openaicompat package which targets Ollama's OpenAI-compatible /v1 endpoint.
const OllamaServerURL = "http://localhost:11434"

// Provider is a base.ModelProvider that builds langchaingo-backed models. It
// supports two backends: Google AI (keyed, via the GenAI API) and a local
// Ollama runtime (keyless, via Ollama's native /api/chat API). The backend is
// fixed at construction by NewProvider / NewOllamaProvider.
type Provider struct {
	vendor    base.LLMVendor
	keys      hippo.KeyProvider // Google AI only
	serverURL string            // Ollama only
	tracer    hippo.Tracer
}

var _ base.ModelProvider = (*Provider)(nil)

// Option configures a Provider.
type Option func(*Provider)

// WithTracer sets the tracer used for model spans. The default is a no-op tracer.
func WithTracer(tracer hippo.Tracer) Option {
	return func(p *Provider) { p.tracer = tracer }
}

// NewProvider creates a Google AI model provider backed by langchaingo.
func NewProvider(keys hippo.KeyProvider, opts ...Option) *Provider {
	p := &Provider{
		vendor: hippo.LLMVendorGoogleAI,
		keys:   keys,
		tracer: hippo.NoopTracer{},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// NewOllamaProvider creates a provider for a local Ollama server, backed by
// langchaingo's native Ollama client. serverURL is the native API base URL
// (e.g. "http://localhost:11434", no /v1 suffix); empty defaults to
// OllamaServerURL. No API key is required.
//
// Prefer this provider over openaicompat.Ollama when you want langchaingo to
// control extended thinking: WithThinking maps to Ollama's native `think`
// reasoning toggle, which the OpenAI-compatible endpoint does not expose.
func NewOllamaProvider(serverURL string, opts ...Option) *Provider {
	if serverURL == "" {
		serverURL = OllamaServerURL
	}
	p := &Provider{
		vendor:    hippo.LLMVendorOllama,
		serverURL: serverURL,
		tracer:    hippo.NoopTracer{},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Model returns a configured model for the given name and LLM type.
func (p *Provider) Model(name string, llmType base.LLMType) (base.Model, error) {
	if llmType == nil || llmType.String() == "" {
		return nil, fmt.Errorf("nil or empty LLM type")
	}

	model := &langchainModel{
		name:      name,
		llmType:   llmType,
		llmVendor: p.vendor,
		serverURL: p.serverURL,
		tracer:    p.tracer,
	}

	switch p.vendor {
	case hippo.LLMVendorOllama:
		// Ollama model names are arbitrary; accept any string verbatim.
	default: // Google AI
		// Accept any model name; reject only a known other-vendor model.
		if v := llmType.Vendor(); v != nil && v.String() != hippo.LLMVendorGoogleAI.String() {
			return nil, fmt.Errorf("langchain provider got a %s model: %q", v.String(), llmType.String())
		}
		apiKey, err := p.keys.APIKey(context.Background(), hippo.LLMVendorGoogleAI)
		if err != nil {
			return nil, fmt.Errorf("no API key for Google AI: %w", err)
		}
		model.apiKey = apiKey
	}

	if err := model.initClient(); err != nil {
		return nil, fmt.Errorf("failed to initialize LLM client: %w", err)
	}
	return model, nil
}
