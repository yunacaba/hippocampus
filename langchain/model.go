package langchain

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/base"
)

// langchainModel is a base.Model backed by langchaingo. This adapter is used
// only for Google AI; the OpenAI and Anthropic vendors have their own
// direct-SDK adapters.
type langchainModel struct {
	name      string
	llmType   base.LLMType
	llmVendor base.LLMVendor
	apiKey    string
	tracer    hippo.Tracer
	model     llms.Model
}

var _ base.Model = &langchainModel{}

// initClient initializes the underlying langchaingo client. Only Google AI is
// supported by this adapter.
func (m *langchainModel) initClient() error {
	httpClient := &http.Client{
		Timeout: 120 * time.Second,
	}

	if m.llmVendor.String() != hippo.LLMVendorGoogleAI.String() {
		return fmt.Errorf("langchain adapter only supports Google AI, got vendor %q", m.llmVendor.String())
	}

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

func (m *langchainModel) Name() string {
	return m.name
}

func (m *langchainModel) LLMType() base.LLMType {
	return m.llmType
}

func (m *langchainModel) LLMVendor() base.LLMVendor {
	return m.llmVendor
}

func (m *langchainModel) Generate(
	ctx context.Context,
	request base.ModelCallRequest,
) (*base.ModelCallResponse, error) {
	ctx, span := m.tracer.StartSpan(ctx, fmt.Sprintf("%s.Model.Generate", m.name))
	defer span.End()

	metrics := &base.ModelCallMetrics{StartTime: time.Now()}
	m.logPreflight(span, request, metrics)

	co := base.ResolveCallOptions(request.Options)

	streamingFunc := request.StreamingFunc
	if streamingFunc != nil {
		streamingFunc = m.wrapStreamingFunc(span, metrics, streamingFunc)
	}
	options := optionsToLangchain(co, streamingFunc)

	if hippo.IsDebugEnabled() {
		hippo.LogPromptDebug(ctx, m.name, m.llmType.String(), request)
	}

	start := time.Now()
	completion, err := m.model.GenerateContent(ctx, messagesToLangchain(request.Messages), options...)
	duration := time.Since(start)

	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("LLM call to %s timed out after %v: %w (context error: %v)",
				m.llmType.String(), duration, err, ctx.Err())
		} else {
			err = fmt.Errorf("LLM call to %s failed after %v: %w",
				m.llmType.String(), duration, err)
		}
		log.Printf("LLM error: %v", err)
		span.RecordError(err)
		return nil, err
	}

	response := responseFromLangchain(completion)
	m.logPostflight(span, completion, metrics)
	response.Metrics = metrics

	if hippo.IsDebugEnabled() {
		hippo.LogResponseDebug(ctx, m.name, m.llmType.String(), response, metrics)
	}

	return response, nil
}

func (m *langchainModel) logPreflight(
	span hippo.Span,
	request base.ModelCallRequest,
	metrics *base.ModelCallMetrics,
) {
	metrics.PromptLength = request.Length()
	metrics.MessageCount = len(request.Messages)
	span.SetAttributes(
		hippo.IntAttr("prompt.length", metrics.PromptLength),
		hippo.IntAttr("prompt.message_count", metrics.MessageCount),
		hippo.StringAttr("model.name", m.name),
		hippo.StringAttr("model.llm_type", m.llmType.String()),
	)
	span.AddEvent(
		"request_start",
		hippo.StringAttr("timestamp", metrics.StartTime.Format(time.RFC3339Nano)),
	)
}

func (m *langchainModel) wrapStreamingFunc(
	span hippo.Span,
	metrics *base.ModelCallMetrics,
	streamingFunc func(ctx context.Context, chunk []byte) error,
) func(ctx context.Context, chunk []byte) error {
	return func(ctx context.Context, chunk []byte) error {
		if !metrics.IsStreaming {
			ttft := time.Since(metrics.StartTime)
			metrics.StreamingTimeToFirstToken = ttft
			span.AddEvent(
				"first_token_received",
				hippo.Int64Attr("ttft.ms", ttft.Milliseconds()),
				hippo.IntAttr("first_chunk.bytes", len(chunk)),
			)
			metrics.IsStreaming = true
		}
		if streamingFunc != nil {
			return streamingFunc(ctx, chunk)
		}
		return nil
	}
}

func (m *langchainModel) logPostflight(
	span hippo.Span,
	completion *llms.ContentResponse,
	metrics *base.ModelCallMetrics,
) {
	if completion == nil || len(completion.Choices) == 0 {
		return
	}
	choice := completion.Choices[0]
	metrics.ResponseLength = len(choice.Content)
	totalDuration := time.Since(metrics.StartTime)
	metrics.TotalDuration = totalDuration

	attrs := []hippo.Attribute{
		hippo.Int64Attr("total.duration.ms", totalDuration.Milliseconds()),
		hippo.IntAttr("response.length", len(choice.Content)),
		hippo.BoolAttr("streaming", metrics.IsStreaming),
	}
	if metrics.IsStreaming {
		duration := totalDuration - metrics.StreamingTimeToFirstToken
		metrics.StreamingDuration = duration
		attrs = append(attrs, hippo.Int64Attr("streaming.duration.ms", duration.Milliseconds()))
	}

	// Token counts, when the vendor surfaces them in GenerationInfo.
	if inputTokens, ok := choice.GenerationInfo["InputTokens"]; ok {
		if it, ok := inputTokens.(int); ok {
			metrics.InputTokens = it
			attrs = append(attrs, hippo.IntAttr("input.tokens", it))
		}
	}
	if outputTokens, ok := choice.GenerationInfo["OutputTokens"]; ok {
		if ot, ok := outputTokens.(int); ok {
			metrics.OutputTokens = ot
			attrs = append(attrs, hippo.IntAttr("output.tokens", ot))
		}
	}

	span.SetAttributes(attrs...)
	span.AddEvent("request_complete", attrs...)
}
