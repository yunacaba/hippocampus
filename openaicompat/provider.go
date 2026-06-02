// Package openaicompat provides model providers for OpenAI-compatible servers —
// chiefly local runtimes like Ollama and LM Studio. It is a thin convenience
// wrapper over the openai adapter: it points the client at a local base URL,
// supplies a placeholder API key (local servers ignore it), and accepts
// arbitrary model names (gemma3, qwen2.5, …) which need no predefined constant.
//
// Structured-output schema enforcement is off by default, because
// OpenAI-compatible servers vary in whether they honor response_format
// json_schema. Enable it with WithResponseSchemaSupport(true) for a server that
// constrains output to the schema (recent Ollama, vLLM, llama.cpp); otherwise
// agents rely on prompt guidance plus the tolerant jsonx parser.
package openaicompat

import (
	"context"

	"github.com/openai/openai-go/v2/option"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/base"
	"github.com/yunacaba/hippocampus/openai"
)

// Default base URLs for common local runtimes' OpenAI-compatible endpoints.
const (
	OllamaBaseURL   = "http://localhost:11434/v1"
	LMStudioBaseURL = "http://localhost:1234/v1"
)

type config struct {
	apiKey         string
	tracer         hippo.Tracer
	responseSchema bool
	reqOpts        []option.RequestOption
}

// Option configures an OpenAI-compatible provider.
type Option func(*config)

// WithAPIKey sets the API key sent to the server. Local servers ignore it; set
// it for an authenticated proxy or gateway. Defaults to a placeholder.
func WithAPIKey(key string) Option {
	return func(c *config) { c.apiKey = key }
}

// WithTracer sets the tracer used for model spans. The default is a no-op tracer.
func WithTracer(tracer hippo.Tracer) Option {
	return func(c *config) { c.tracer = tracer }
}

// WithResponseSchemaSupport declares that the server honors response_format
// json_schema, so the agent will send structured-output schemas to it. Off by
// default.
func WithResponseSchemaSupport(supported bool) Option {
	return func(c *config) { c.responseSchema = supported }
}

// WithRequestOptions appends OpenAI SDK request options (e.g. custom headers or
// HTTP client). Useful for proxies and testing.
func WithRequestOptions(opts ...option.RequestOption) Option {
	return func(c *config) { c.reqOpts = append(c.reqOpts, opts...) }
}

// NewProvider builds an OpenAI-compatible provider pointed at baseURL (which
// should include the API version path, e.g. ".../v1"). Model names are passed
// through verbatim as the wire model id.
func NewProvider(baseURL string, opts ...Option) base.ModelProvider {
	cfg := config{
		apiKey:         "hippocampus-local", // placeholder; local servers ignore it
		tracer:         hippo.NoopTracer{},
		responseSchema: false,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	reqOpts := append([]option.RequestOption{option.WithBaseURL(baseURL)}, cfg.reqOpts...)
	return openai.NewProvider(
		staticKey(cfg.apiKey),
		openai.WithTracer(cfg.tracer),
		openai.WithResponseSchemaSupport(cfg.responseSchema),
		openai.WithRequestOptions(reqOpts...),
	)
}

// Ollama builds a provider for a local Ollama server (OllamaBaseURL by default;
// override with WithRequestOptions(option.WithBaseURL(...)) for a remote host).
func Ollama(opts ...Option) base.ModelProvider {
	return NewProvider(OllamaBaseURL, opts...)
}

// LMStudio builds a provider for a local LM Studio server.
func LMStudio(opts ...Option) base.ModelProvider {
	return NewProvider(LMStudioBaseURL, opts...)
}

// staticKey is a KeyProvider that returns a fixed key for any vendor.
type staticKey string

func (k staticKey) APIKey(context.Context, base.LLMVendor) (string, error) {
	return string(k), nil
}
