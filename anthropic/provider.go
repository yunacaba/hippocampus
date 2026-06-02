package anthropic

import (
	"context"
	"fmt"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/base"
)

// Provider is a base.ModelProvider that builds direct-SDK Anthropic models. API
// keys are obtained from a hippocampus.KeyProvider.
type Provider struct {
	keys    hippo.KeyProvider
	tracer  hippo.Tracer
	reqOpts []option.RequestOption
}

var _ base.ModelProvider = (*Provider)(nil)

// Option configures a Provider.
type Option func(*Provider)

// WithTracer sets the tracer used for model spans. The default is a no-op tracer.
func WithTracer(tracer hippo.Tracer) Option {
	return func(p *Provider) { p.tracer = tracer }
}

// WithRequestOptions appends Anthropic SDK request options applied to every
// client (e.g. a custom HTTP client or base URL). Useful for testing and proxies.
func WithRequestOptions(opts ...option.RequestOption) Option {
	return func(p *Provider) { p.reqOpts = append(p.reqOpts, opts...) }
}

// NewProvider creates an Anthropic model provider.
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
// must be an Anthropic model.
func (p *Provider) Model(name string, llmType base.LLMType) (base.Model, error) {
	if llmType == nil || llmType.String() == "" {
		return nil, fmt.Errorf("nil or empty LLM type")
	}
	// Accept any model name; reject only a known other-vendor model.
	if v := llmType.Vendor(); v != nil && v.String() != hippo.LLMVendorAnthropic.String() {
		return nil, fmt.Errorf("anthropic provider got a %s model: %q", v.String(), llmType.String())
	}

	apiKey, err := p.keys.APIKey(context.Background(), hippo.LLMVendorAnthropic)
	if err != nil {
		return nil, fmt.Errorf("no API key for Anthropic: %w", err)
	}

	reqOpts := append([]option.RequestOption{option.WithAPIKey(apiKey)}, p.reqOpts...)
	client := sdk.NewClient(reqOpts...)

	return &anthropicModel{
		name:      name,
		llmType:   llmType,
		llmVendor: hippo.LLMVendorAnthropic,
		tracer:    p.tracer,
		client:    client,
	}, nil
}
