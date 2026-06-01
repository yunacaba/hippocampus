package anthropic

import (
	"context"
	"fmt"
	"log"
	"time"

	sdk "github.com/anthropics/anthropic-sdk-go"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/base"
)

// anthropicModel is a base.Model backed by the official Anthropic Go SDK. It
// forwards the end-user account ID set via hippocampus.WithUserID as the
// request's metadata.user_id field.
type anthropicModel struct {
	name      string
	llmType   base.LLMType
	llmVendor base.LLMVendor
	tracer    hippo.Tracer
	client    sdk.Client
}

var _ base.Model = (*anthropicModel)(nil)

func (m *anthropicModel) Name() string              { return m.name }
func (m *anthropicModel) LLMType() base.LLMType     { return m.llmType }
func (m *anthropicModel) LLMVendor() base.LLMVendor { return m.llmVendor }

func (m *anthropicModel) Generate(
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
	system, msgs := splitMessages(request.Messages)
	params := sdk.MessageNewParams{
		Model:    sdk.Model(m.llmType.String()),
		Messages: msgs,
		System:   system,
	}
	applyOptions(&params, co)

	// End-user attribution: forward the account ID as metadata.user_id.
	if userID, ok := hippo.UserIDFromContext(ctx); ok && userID != "" {
		params.Metadata = sdk.MetadataParam{UserID: sdk.String(userID)}
		span.SetAttributes(hippo.StringAttr("llm.user_id", userID))
	}

	if hippo.IsDebugEnabled() {
		hippo.LogPromptDebug(ctx, m.name, m.llmType.String(), request)
	}

	start := time.Now()
	var (
		message *sdk.Message
		err     error
	)
	if request.StreamingFunc != nil {
		message, err = m.generateStreaming(ctx, span, params, metrics, request.StreamingFunc)
	} else {
		message, err = m.client.Messages.New(ctx, params)
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

	response := responseFromAnthropic(message)
	m.recordMetrics(span, message, metrics)
	response.Metrics = metrics

	if hippo.IsDebugEnabled() {
		hippo.LogResponseDebug(ctx, m.name, m.llmType.String(), response, metrics)
	}

	return response, nil
}

// generateStreaming consumes the SSE stream, forwarding text deltas to the
// streaming function and recording time-to-first-token, then returns the fully
// accumulated message.
func (m *anthropicModel) generateStreaming(
	ctx context.Context,
	span hippo.Span,
	params sdk.MessageNewParams,
	metrics *base.ModelCallMetrics,
	streamingFunc func(ctx context.Context, chunk []byte) error,
) (*sdk.Message, error) {
	stream := m.client.Messages.NewStreaming(ctx, params)
	defer stream.Close()

	var message sdk.Message
	for stream.Next() {
		event := stream.Current()
		if err := message.Accumulate(event); err != nil {
			return nil, err
		}

		delta := event.Delta.Text
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
	return &message, nil
}

func (m *anthropicModel) recordMetrics(
	span hippo.Span,
	message *sdk.Message,
	metrics *base.ModelCallMetrics,
) {
	metrics.TotalDuration = time.Since(metrics.StartTime)
	if message != nil {
		metrics.InputTokens = int(message.Usage.InputTokens)
		metrics.OutputTokens = int(message.Usage.OutputTokens)
		for _, block := range message.Content {
			if block.Type == "text" {
				metrics.ResponseLength += len(block.Text)
			}
		}
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
