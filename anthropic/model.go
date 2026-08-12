package anthropic

import (
	"context"
	"net/http"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/base"
)

// requestIDHeader is Anthropic's per-call identifier. Sent lowercase on the
// wire; named canonically here because http.Header.Get canonicalizes.
const requestIDHeader = "Request-Id"

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

// SupportsResponseSchema reports that Anthropic can enforce a response schema
// (via a forced output tool when the call has no other tools).
func (m *anthropicModel) SupportsResponseSchema() bool { return true }

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

			// Decide streaming vs non-streaming. A caller-supplied StreamingFunc
			// always streams. Otherwise, the SDK rejects a non-streaming
			// Messages.New whose max_tokens implies a >10min run (e.g. 64k
			// tokens → a ~30min estimate) with "streaming is required for
			// operations that may take longer than 10 minutes". Ask the SDK's
			// own guard: if New would be rejected, stream-and-accumulate
			// instead (the streaming endpoint has no such limit) and return the
			// same fully-accumulated message. Using the SDK function keeps this
			// in lockstep with whatever threshold the SDK enforces.
			useStreaming := request.StreamingFunc != nil
			if !useStreaming {
				if _, terr := sdk.CalculateNonStreamingTimeout(int(params.MaxTokens), params.Model, nil); terr != nil {
					useStreaming = true
				}
			}

			// Capture the HTTP response so the call's Request-Id can be
			// reported. Per-call rather than client-wide middleware: the client
			// is shared across concurrent calls, so a single shared destination
			// would race and hand a caller another call's id — worse than none,
			// since this value's whole purpose is correlating one specific call.
			var httpResp *http.Response
			capture := option.WithResponseInto(&httpResp)

			var (
				message *sdk.Message
				err     error
			)
			if useStreaming {
				message, err = m.generateStreaming(ctx, span, params, metrics, request.StreamingFunc, capture)
			} else {
				message, err = m.client.Messages.New(ctx, params, capture)
			}

			// On the span before the error check, so the id survives a call that
			// failed after its response arrived. That case reports it nowhere
			// else: a mid-stream connection drop is a transport error rather
			// than an sdk.Error, so it carries no RequestID, and returning below
			// skips GenerationInfo. It is also the case most worth correlating —
			// a managed-analysis run that streams for twenty minutes, spends
			// tokens, and then dies.
			//
			// The span rather than the error, because this is one route to one
			// surface: an investigator looking at a failed call is already here,
			// and wrapping the id into the error would make its type depend on
			// how the call failed.
			requestID := responseRequestID(httpResp)
			if requestID != "" {
				span.SetAttributes(hippo.StringAttr("llm.request_id", requestID))
			}

			if err != nil {
				return nil, err
			}

			resp := responseFromAnthropic(message)

			if requestID != "" {
				resp.GenerationInfo[base.GenerationInfoRequestID] = requestID
			}

			// Structured output via forced tool: lift the synthetic tool's input
			// (which is the schema-conformant JSON) into Content, and drop it from
			// ToolCalls so the agent treats it as the final answer rather than a
			// tool to execute.
			if co.ResponseSchema != nil && len(co.Tools) == 0 && message != nil {
				name := structuredOutputToolName(co.ResponseSchema)
				for _, block := range message.Content {
					if block.Type == "tool_use" && block.Name == name {
						resp.Content = string(block.Input)
						resp.ToolCalls = nil
						break
					}
				}
			}

			if message != nil {
				metrics.InputTokens = int(message.Usage.InputTokens)
				metrics.OutputTokens = int(message.Usage.OutputTokens)
				metrics.CacheReadInputTokens = int(message.Usage.CacheReadInputTokens)
				metrics.CacheCreationInputTokens = int(message.Usage.CacheCreationInputTokens)
				metrics.ResponseLength = len(resp.Content)
			}
			return resp, nil
		})
}

// responseRequestID reads Anthropic's per-call identifier from a response,
// tolerating the absence of either.
//
// A missing header is ordinary rather than exceptional: a proxy may strip it,
// and a test transport is unlikely to invent one. It is reported when present
// and omitted when not, because an empty string recorded as an id would be
// indistinguishable, downstream, from a call that reported one.
func responseRequestID(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	return resp.Header.Get(requestIDHeader)
}

// generateStreaming consumes the SSE stream, forwarding text deltas to the
// streaming function (when non-nil) and recording time-to-first-token, then
// returns the fully accumulated message. A nil streamingFunc is valid: the
// adapter always streams to avoid the SDK's non-streaming 10-minute guard, so
// non-streaming callers run this path too and simply receive the accumulated
// result without per-delta callbacks.
func (m *anthropicModel) generateStreaming(
	ctx context.Context,
	span hippo.Span,
	params sdk.MessageNewParams,
	metrics *base.ModelCallMetrics,
	streamingFunc func(ctx context.Context, chunk []byte) error,
	opts ...option.RequestOption,
) (*sdk.Message, error) {
	stream := m.client.Messages.NewStreaming(ctx, params, opts...)
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
		if text != "" && streamingFunc != nil {
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
