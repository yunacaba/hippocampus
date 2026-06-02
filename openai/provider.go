package openai

import (
	"context"
	"fmt"

	oai "github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/base"
)

// Provider is a base.ModelProvider that builds direct-SDK OpenAI models. API
// keys are obtained from a hippocampus.KeyProvider.
type Provider struct {
	keys           hippo.KeyProvider
	tracer         hippo.Tracer
	reqOpts        []option.RequestOption
	responseSchema bool
}

var _ base.ModelProvider = (*Provider)(nil)

// Option configures a Provider.
type Option func(*Provider)

// WithTracer sets the tracer used for model spans. The default is a no-op tracer.
func WithTracer(tracer hippo.Tracer) Option {
	return func(p *Provider) { p.tracer = tracer }
}

// WithRequestOptions appends OpenAI SDK request options applied to every client
// (e.g. a custom HTTP client or base URL). Useful for testing and proxies.
func WithRequestOptions(opts ...option.RequestOption) Option {
	return func(p *Provider) { p.reqOpts = append(p.reqOpts, opts...) }
}

// WithResponseSchemaSupport declares whether the endpoint honors
// response_format json_schema. The default is true (the real OpenAI API). Set
// it false for an OpenAI-compatible server that doesn't support json_schema, so
// the agent falls back to prompt guidance + tolerant parsing instead of sending
// a request the server would reject.
//
// This is a provider-wide declaration, not per-model: if one provider serves a
// mix of models with differing support (e.g. a current model plus a legacy one
// that only does json_object), set it false and rely on prompt guidance, or use
// separate providers.
func WithResponseSchemaSupport(supported bool) Option {
	return func(p *Provider) { p.responseSchema = supported }
}

// NewProvider creates an OpenAI model provider.
func NewProvider(keys hippo.KeyProvider, opts ...Option) *Provider {
	p := &Provider{
		keys:           keys,
		tracer:         hippo.NoopTracer{},
		responseSchema: true,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Model returns a configured model for the given name and LLM type. The type
// must be an OpenAI model.
func (p *Provider) Model(name string, llmType base.LLMType) (base.Model, error) {
	if llmType == nil || llmType.String() == "" {
		return nil, fmt.Errorf("nil or empty LLM type")
	}
	// Accept any model name, using it as the wire model id — this lets new
	// OpenAI models (and OpenAI-compatible local models) be used without a
	// constant. Reject only a model that is a *known* other vendor, to catch
	// passing e.g. an Anthropic model to the OpenAI provider.
	if v := llmType.Vendor(); v != nil && v.String() != hippo.LLMVendorOpenAI.String() {
		return nil, fmt.Errorf("openai provider got a %s model: %q", v.String(), llmType.String())
	}

	apiKey, err := p.keys.APIKey(context.Background(), hippo.LLMVendorOpenAI)
	if err != nil {
		return nil, fmt.Errorf("no API key for OpenAI: %w", err)
	}

	reqOpts := append([]option.RequestOption{option.WithAPIKey(apiKey)}, p.reqOpts...)
	client := oai.NewClient(reqOpts...)

	return &openaiModel{
		name:                   name,
		llmType:                llmType,
		llmVendor:              hippo.LLMVendorOpenAI,
		supportsResponseSchema: p.responseSchema,
		tracer:                 p.tracer,
		client:                 client,
	}, nil
}
