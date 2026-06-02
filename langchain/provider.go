package langchain

import (
	"context"
	"fmt"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/base"
)

// Provider is a base.ModelProvider that builds langchaingo-backed models for
// Google AI. API keys are obtained from a hippocampus.KeyProvider.
type Provider struct {
	keys   hippo.KeyProvider
	tracer hippo.Tracer
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
		keys:   keys,
		tracer: hippo.NoopTracer{},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Model returns a configured model for the given name and LLM type. The type
// must be a Google AI model.
func (p *Provider) Model(name string, llmType base.LLMType) (base.Model, error) {
	if llmType == nil || llmType.String() == "" {
		return nil, fmt.Errorf("nil or empty LLM type")
	}
	// Accept any model name; reject only a known other-vendor model.
	if v := llmType.Vendor(); v != nil && v.String() != hippo.LLMVendorGoogleAI.String() {
		return nil, fmt.Errorf("langchain provider got a %s model: %q", v.String(), llmType.String())
	}

	apiKey, err := p.keys.APIKey(context.Background(), hippo.LLMVendorGoogleAI)
	if err != nil {
		return nil, fmt.Errorf("no API key for Google AI: %w", err)
	}

	model := &langchainModel{
		name:      name,
		llmType:   llmType,
		llmVendor: hippo.LLMVendorGoogleAI,
		apiKey:    apiKey,
		tracer:    p.tracer,
	}
	if err := model.initClient(); err != nil {
		return nil, fmt.Errorf("failed to initialize LLM client: %w", err)
	}
	return model, nil
}
