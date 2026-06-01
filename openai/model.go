package openai

import (
	"context"
	"fmt"
	"log"
	"time"

	oai "github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/shared"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/base"
)

// openaiModel is a base.Model backed by the official OpenAI Go SDK. It forwards
// the end-user account ID set via hippocampus.WithUserID as the request's
// "user" field.
type openaiModel struct {
	name      string
	llmType   base.LLMType
	llmVendor base.LLMVendor
	tracer    hippo.Tracer
	client    oai.Client
}

var _ base.Model = (*openaiModel)(nil)

func (m *openaiModel) Name() string              { return m.name }
func (m *openaiModel) LLMType() base.LLMType     { return m.llmType }
func (m *openaiModel) LLMVendor() base.LLMVendor { return m.llmVendor }

func (m *openaiModel) Generate(
	ctx context.Context,
	request base.ModelCallRequest,
) (*base.ModelCallResponse, error) {
	ctx, span := m.tracer.StartSpan(ctx, fmt.Sprintf("%s.Model.Generate", m.name))
	defer span.End()

	metrics := &base.ModelCallMetrics{StartTime: time.Now()}
	metrics.PromptLength = request.Length()
	metrics.MessageCount = len(request.Messages)
	span.SetAttributes(
		hippo.IntAttr("prompt.length", metrics.PromptLength),
		hippo.IntAttr("prompt.message_count", metrics.MessageCount),
		hippo.StringAttr("model.name", m.name),
		hippo.StringAttr("model.llm_type", m.llmType.String()),
	)

	co := base.ResolveCallOptions(request.Options)
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

	if hippo.IsDebugEnabled() {
		hippo.LogPromptDebug(ctx, m.name, m.llmType.String(), request)
	}

	start := time.Now()
	var (
		completion *oai.ChatCompletion
		err        error
	)
	if request.StreamingFunc != nil {
		completion, err = m.generateStreaming(ctx, span, params, metrics, request.StreamingFunc)
	} else {
		completion, err = m.client.Chat.Completions.New(ctx, params)
	}
	duration := time.Since(start)

	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("LLM call to %s timed out after %v: %w (context error: %v)",
				m.llmType.String(), duration, err, ctx.Err())
		} else {
			err = fmt.Errorf("LLM call to %s failed after %v: %w", m.llmType.String(), duration, err)
		}
		log.Printf("LLM error: %v", err)
		span.RecordError(err)
		return nil, err
	}

	response := responseFromOpenAI(completion)
	m.recordMetrics(span, completion, metrics)
	response.Metrics = metrics

	if hippo.IsDebugEnabled() {
		hippo.LogResponseDebug(ctx, m.name, m.llmType.String(), response, metrics)
	}

	return response, nil
}

// generateStreaming consumes the SSE stream, forwarding content deltas to the
// streaming function and recording time-to-first-token, then returns the fully
// accumulated completion.
func (m *openaiModel) generateStreaming(
	ctx context.Context,
	span hippo.Span,
	params oai.ChatCompletionNewParams,
	metrics *base.ModelCallMetrics,
	streamingFunc func(ctx context.Context, chunk []byte) error,
) (*oai.ChatCompletion, error) {
	stream := m.client.Chat.Completions.NewStreaming(ctx, params)
	defer stream.Close()

	var acc oai.ChatCompletionAccumulator
	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)

		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if delta == "" {
			continue
		}
		if !metrics.IsStreaming {
			ttft := time.Since(metrics.StartTime)
			metrics.StreamingTimeToFirstToken = ttft
			metrics.IsStreaming = true
			span.AddEvent(
				"first_token_received",
				hippo.Int64Attr("ttft.ms", ttft.Milliseconds()),
				hippo.IntAttr("first_chunk.bytes", len(delta)),
			)
		}
		if err := streamingFunc(ctx, []byte(delta)); err != nil {
			return nil, err
		}
	}
	if err := stream.Err(); err != nil {
		return nil, err
	}
	completion := acc.ChatCompletion
	return &completion, nil
}

func (m *openaiModel) recordMetrics(
	span hippo.Span,
	completion *oai.ChatCompletion,
	metrics *base.ModelCallMetrics,
) {
	metrics.TotalDuration = time.Since(metrics.StartTime)
	if completion != nil && len(completion.Choices) > 0 {
		metrics.ResponseLength = len(completion.Choices[0].Message.Content)
		metrics.InputTokens = int(completion.Usage.PromptTokens)
		metrics.OutputTokens = int(completion.Usage.CompletionTokens)
	}
	attrs := []hippo.Attribute{
		hippo.Int64Attr("total.duration.ms", metrics.TotalDuration.Milliseconds()),
		hippo.IntAttr("response.length", metrics.ResponseLength),
		hippo.BoolAttr("streaming", metrics.IsStreaming),
		hippo.IntAttr("input.tokens", metrics.InputTokens),
		hippo.IntAttr("output.tokens", metrics.OutputTokens),
	}
	if metrics.IsStreaming {
		metrics.StreamingDuration = metrics.TotalDuration - metrics.StreamingTimeToFirstToken
		attrs = append(attrs, hippo.Int64Attr("streaming.duration.ms", metrics.StreamingDuration.Milliseconds()))
	}
	span.SetAttributes(attrs...)
	span.AddEvent("request_complete", attrs...)
}
