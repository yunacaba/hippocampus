package openai

import (
	"context"

	oai "github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/shared"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/base"
)

// openaiModel is a base.Model backed by the official OpenAI Go SDK. It forwards
// the end-user account ID set via hippocampus.WithUserID as the request's
// "user" field.
type openaiModel struct {
	name                   string
	llmType                base.LLMType
	llmVendor              base.LLMVendor
	supportsResponseSchema bool
	tracer                 hippo.Tracer
	client                 oai.Client
}

var _ base.Model = (*openaiModel)(nil)

func (m *openaiModel) Name() string              { return m.name }
func (m *openaiModel) LLMType() base.LLMType     { return m.llmType }
func (m *openaiModel) LLMVendor() base.LLMVendor { return m.llmVendor }

// SupportsResponseSchema reports whether the endpoint honors response_format
// json_schema (true for the real OpenAI API; configurable for compatible
// servers via WithResponseSchemaSupport).
func (m *openaiModel) SupportsResponseSchema() bool { return m.supportsResponseSchema }

func (m *openaiModel) Generate(
	ctx context.Context,
	request base.ModelCallRequest,
) (*base.ModelCallResponse, error) {
	return hippo.RunModelGenerate(ctx, m.tracer, m.name, m.llmType.String(), request,
		func(ctx context.Context, co base.CallOptions, span hippo.Span, metrics *base.ModelCallMetrics) (*base.ModelCallResponse, error) {
			params := oai.ChatCompletionNewParams{
				Model:    shared.ChatModel(m.llmType.String()),
				Messages: messagesToOpenAI(request.Messages),
			}
			applyOptions(&params, co)

			// End-user attribution: forward the account ID as the "user" field.
			if userID, ok := hippo.UserIDFromContext(ctx); ok && userID != "" {
				params.User = oai.String(userID)
				span.SetAttributes(hippo.StringAttr("llm.user_id", userID))
			}

			var (
				completion *oai.ChatCompletion
				err        error
			)
			if request.StreamingFunc != nil {
				completion, err = m.generateStreaming(ctx, span, params, metrics, request.StreamingFunc)
			} else {
				completion, err = m.client.Chat.Completions.New(ctx, params)
			}
			if err != nil {
				return nil, err
			}

			resp := responseFromOpenAI(completion)
			if completion != nil && len(completion.Choices) > 0 {
				metrics.ResponseLength = len(completion.Choices[0].Message.Content)
				metrics.InputTokens = int(completion.Usage.PromptTokens)
				metrics.OutputTokens = int(completion.Usage.CompletionTokens)
			}
			return resp, nil
		})
}

// generateStreaming consumes the SSE stream, forwarding content deltas to the
// streaming function and recording time-to-first-token, then returns the fully
// accumulated completion. IncludeUsage is requested so the accumulated
// completion carries token usage (otherwise streamed calls report zero tokens).
func (m *openaiModel) generateStreaming(
	ctx context.Context,
	span hippo.Span,
	params oai.ChatCompletionNewParams,
	metrics *base.ModelCallMetrics,
	streamingFunc func(ctx context.Context, chunk []byte) error,
) (*oai.ChatCompletion, error) {
	params.StreamOptions = oai.ChatCompletionStreamOptionsParam{IncludeUsage: oai.Bool(true)}

	stream := m.client.Chat.Completions.NewStreaming(ctx, params)
	defer stream.Close()

	var acc oai.ChatCompletionAccumulator
	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)

		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		// Mark first token on any content or tool-call delta, so tool-only
		// streamed turns still record TTFT.
		if delta.Content != "" || len(delta.ToolCalls) > 0 {
			hippo.MarkFirstToken(span, metrics, len(delta.Content))
		}
		if delta.Content != "" {
			if err := streamingFunc(ctx, []byte(delta.Content)); err != nil {
				return nil, err
			}
		}
	}
	if err := stream.Err(); err != nil {
		return nil, err
	}
	completion := acc.ChatCompletion
	return &completion, nil
}
