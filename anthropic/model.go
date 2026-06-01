package anthropic

import (
	"context"

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
	return hippo.RunModelGenerate(ctx, m.tracer, m.name, m.llmType.String(), request,
		func(ctx context.Context, co base.CallOptions, span hippo.Span, metrics *base.ModelCallMetrics) (*base.ModelCallResponse, error) {
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

			var (
				message *sdk.Message
				err     error
			)
			if request.StreamingFunc != nil {
				message, err = m.generateStreaming(ctx, span, params, metrics, request.StreamingFunc)
			} else {
				message, err = m.client.Messages.New(ctx, params)
			}
			if err != nil {
				return nil, err
			}

			resp := responseFromAnthropic(message)
			if message != nil {
				metrics.InputTokens = int(message.Usage.InputTokens)
				metrics.OutputTokens = int(message.Usage.OutputTokens)
				metrics.ResponseLength = len(resp.Content)
			}
			return resp, nil
		})
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

		// Mark first token on any content or tool-input delta, so tool-only
		// streamed turns still record TTFT.
		text := event.Delta.Text
		if text != "" || event.Delta.PartialJSON != "" {
			hippo.MarkFirstToken(span, metrics, len(text))
		}
		if text != "" {
			if err := streamingFunc(ctx, []byte(text)); err != nil {
				return nil, err
			}
		}
	}
	if err := stream.Err(); err != nil {
		return nil, err
	}
	return &message, nil
}
