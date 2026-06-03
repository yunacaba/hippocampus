package langchain

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/ollama"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/base"
)

// langchainModel is a base.Model backed by langchaingo. It serves Google AI
// (keyed GenAI API) and local Ollama (keyless native API); the OpenAI and
// Anthropic vendors have their own direct-SDK adapters.
type langchainModel struct {
	name      string
	llmType   base.LLMType
	llmVendor base.LLMVendor
	apiKey    string // Google AI
	serverURL string // Ollama
	tracer    hippo.Tracer
	model     llms.Model
}

var _ base.Model = &langchainModel{}

// initClient initializes the underlying langchaingo client for the model's
// vendor.
func (m *langchainModel) initClient() error {
	httpClient := &http.Client{
		Timeout: 120 * time.Second,
	}

	switch m.llmVendor.String() {
	case hippo.LLMVendorOllama.String():
		model, err := ollama.New(
			ollama.WithModel(m.llmType.String()),
			ollama.WithServerURL(m.serverURL),
			ollama.WithHTTPClient(httpClient),
		)
		if err != nil {
			return fmt.Errorf("failed to create %s client: %w", m.llmVendor.String(), err)
		}
		m.model = model
		return nil
	default: // Google AI
		ctx := context.Background()
		model, err := googleai.New(
			ctx,
			googleai.WithAPIKey(m.apiKey),
			googleai.WithDefaultModel(m.llmType.String()),
			googleai.WithHTTPClient(httpClient),
		)
		if err != nil {
			return fmt.Errorf("failed to create %s client: %w", m.llmVendor.String(), err)
		}
		m.model = model
		return nil
	}
}

func (m *langchainModel) Name() string              { return m.name }
func (m *langchainModel) LLMType() base.LLMType     { return m.llmType }
func (m *langchainModel) LLMVendor() base.LLMVendor { return m.llmVendor }

// SupportsResponseSchema reports false: Google AI via langchaingo exposes only
// a JSON MIME mode, not schema enforcement, so the agent relies on prompt
// guidance + the tolerant jsonx parser instead.
func (m *langchainModel) SupportsResponseSchema() bool { return false }

func (m *langchainModel) Generate(
	ctx context.Context,
	request base.ModelCallRequest,
) (*base.ModelCallResponse, error) {
	return hippo.RunModelGenerate(ctx, m.tracer, m.name, m.llmType.String(), request,
		func(ctx context.Context, co base.CallOptions, span hippo.Span, metrics *base.ModelCallMetrics) (*base.ModelCallResponse, error) {
			streamingFunc := request.StreamingFunc
			if streamingFunc != nil {
				streamingFunc = m.wrapStreamingFunc(span, metrics, streamingFunc)
			}
			options := optionsToLangchain(co, streamingFunc)

			completion, err := m.model.GenerateContent(ctx, messagesToLangchain(request.Messages), options...)
			if err != nil {
				return nil, err
			}

			resp := responseFromLangchain(completion)
			if completion != nil && len(completion.Choices) > 0 {
				choice := completion.Choices[0]
				metrics.ResponseLength = len(choice.Content)
				// Token counts, when the vendor surfaces them in GenerationInfo.
				if it, ok := choice.GenerationInfo["InputTokens"].(int); ok {
					metrics.InputTokens = it
				}
				if ot, ok := choice.GenerationInfo["OutputTokens"].(int); ok {
					metrics.OutputTokens = ot
				}
			}
			return resp, nil
		})
}

// wrapStreamingFunc records time-to-first-token before delegating to the
// caller's streaming function.
func (m *langchainModel) wrapStreamingFunc(
	span hippo.Span,
	metrics *base.ModelCallMetrics,
	streamingFunc func(ctx context.Context, chunk []byte) error,
) func(ctx context.Context, chunk []byte) error {
	return func(ctx context.Context, chunk []byte) error {
		hippo.MarkFirstToken(span, metrics, len(chunk))
		return streamingFunc(ctx, chunk)
	}
}
