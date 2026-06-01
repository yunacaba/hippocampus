package hippocampus

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yunacaba/hippocampus/base"
)

// GenerateCall performs the provider-specific half of a model call. It receives
// the resolved call options, the active span, and a metrics struct it should
// populate (InputTokens, OutputTokens, ResponseLength; and, for streaming, call
// MarkFirstToken on the first chunk). It returns the owned response with
// Metrics left unset — RunModelGenerate fills that in.
type GenerateCall func(
	ctx context.Context,
	opts CallOptions,
	span Span,
	metrics *ModelCallMetrics,
) (*ModelCallResponse, error)

// RunModelGenerate wraps a provider call with the span, metrics, debug logging,
// and error handling shared by every base.Model adapter, so adapter authors
// implement only the provider-specific GenerateCall. modelType is the model's
// wire name (used in spans, debug output, and error messages).
func RunModelGenerate(
	ctx context.Context,
	tracer Tracer,
	name string,
	modelType string,
	request ModelCallRequest,
	call GenerateCall,
) (*ModelCallResponse, error) {
	if tracer == nil {
		tracer = NoopTracer{}
	}
	ctx, span := tracer.StartSpan(ctx, fmt.Sprintf("%s.Model.Generate", name))
	defer span.End()

	metrics := &ModelCallMetrics{StartTime: time.Now()}
	metrics.PromptLength = request.Length()
	metrics.MessageCount = len(request.Messages)
	span.SetAttributes(
		IntAttr("prompt.length", metrics.PromptLength),
		IntAttr("prompt.message_count", metrics.MessageCount),
		StringAttr("model.name", name),
		StringAttr("model.llm_type", modelType),
	)

	opts := base.ResolveCallOptions(request.Options)

	if IsDebugEnabled() {
		LogPromptDebug(ctx, name, modelType, request)
	}

	start := time.Now()
	resp, err := call(ctx, opts, span, metrics)
	duration := time.Since(start)

	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("LLM call to %s timed out after %v: %w (context error: %v)",
				modelType, duration, err, ctx.Err())
		} else {
			err = fmt.Errorf("LLM call to %s failed after %v: %w", modelType, duration, err)
		}
		log.Printf("LLM error: %v", err)
		span.RecordError(err)
		return nil, err
	}

	finishMetrics(span, metrics)
	if resp != nil {
		resp.Metrics = metrics
	}

	if IsDebugEnabled() {
		LogResponseDebug(ctx, name, modelType, resp, metrics)
	}

	return resp, nil
}

// MarkFirstToken records time-to-first-token the first time it is called for a
// given metrics struct, and flips the streaming flag. Adapters call it on the
// first streamed chunk of any kind (text or tool-call delta).
func MarkFirstToken(span Span, metrics *ModelCallMetrics, chunkBytes int) {
	if metrics.IsStreaming {
		return
	}
	ttft := time.Since(metrics.StartTime)
	metrics.StreamingTimeToFirstToken = ttft
	metrics.IsStreaming = true
	span.AddEvent(
		"first_token_received",
		Int64Attr("ttft.ms", ttft.Milliseconds()),
		IntAttr("first_chunk.bytes", chunkBytes),
	)
}

// finishMetrics records the post-call duration, streaming, and token attributes.
func finishMetrics(span Span, metrics *ModelCallMetrics) {
	metrics.TotalDuration = time.Since(metrics.StartTime)
	attrs := []Attribute{
		Int64Attr("total.duration.ms", metrics.TotalDuration.Milliseconds()),
		IntAttr("response.length", metrics.ResponseLength),
		BoolAttr("streaming", metrics.IsStreaming),
		IntAttr("input.tokens", metrics.InputTokens),
		IntAttr("output.tokens", metrics.OutputTokens),
	}
	if metrics.IsStreaming {
		metrics.StreamingDuration = metrics.TotalDuration - metrics.StreamingTimeToFirstToken
		attrs = append(attrs, Int64Attr("streaming.duration.ms", metrics.StreamingDuration.Milliseconds()))
	}
	span.SetAttributes(attrs...)
	span.AddEvent("request_complete", attrs...)
}
