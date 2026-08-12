package anthropic_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"testing/iotest"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/anthropic"
	"github.com/yunacaba/hippocampus/base"
)

// Anthropic identifies each call with a Request-Id response header, and it is
// the only handle a provider-side notice cites. Reporting it lets a caller
// correlate one specific call against provider records; dropping it, as this
// adapter previously did, makes that correlation impossible after the fact.

// requestIDTransport returns a canned message, optionally carrying a
// Request-Id, over either the streaming or non-streaming endpoint.
type requestIDTransport struct {
	requestID string
	streaming bool
}

func (tr *requestIDTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	header := http.Header{}
	if tr.requestID != "" {
		// Lowercase, as Anthropic sends it — http.Header.Get canonicalizes, and
		// a test that wrote the canonical form would not prove that.
		header.Set("request-id", tr.requestID)
	}

	body := `{"id":"msg_x","type":"message","role":"assistant","model":"claude-haiku-4-5-20251001",` +
		`"content":[{"type":"text","text":"hi"}],"stop_reason":"end_turn","stop_sequence":null,` +
		`"usage":{"input_tokens":1,"output_tokens":1}}`
	header.Set("Content-Type", "application/json")

	if tr.streaming {
		header.Set("Content-Type", "text/event-stream")
		body = "event: message_start\n" +
			`data: {"type":"message_start","message":{"id":"msg_x","type":"message","role":"assistant",` +
			`"model":"claude-haiku-4-5-20251001","content":[],"stop_reason":null,"stop_sequence":null,` +
			`"usage":{"input_tokens":1,"output_tokens":0}}}` + "\n\n" +
			"event: content_block_start\n" +
			`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n\n" +
			"event: content_block_delta\n" +
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}` + "\n\n" +
			"event: content_block_stop\n" +
			`data: {"type":"content_block_stop","index":0}` + "\n\n" +
			"event: message_delta\n" +
			`data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},` +
			`"usage":{"output_tokens":1}}` + "\n\n" +
			"event: message_stop\n" +
			`data: {"type":"message_stop"}` + "\n\n"
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     header,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Request:    req,
	}, nil
}

func requestIDModel(t *testing.T, tr http.RoundTripper) base.Model {
	t.Helper()
	provider := anthropic.NewProvider(
		staticKeyProvider{key: "test-key"},
		anthropic.WithRequestOptions(option.WithHTTPClient(&http.Client{Transport: tr})),
	)
	model, err := provider.Model("requestid_test", hippo.AnthropicClaudeHaiku45)
	if err != nil {
		t.Fatalf("build model: %v", err)
	}
	return model
}

func TestGenerateReportsTheRequestID(t *testing.T) {
	tr := &requestIDTransport{requestID: "req_011CQtest"}
	resp, err := requestIDModel(t, tr).Generate(context.Background(), userReq())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	got, ok := resp.GenerationInfo[base.GenerationInfoRequestID]
	if !ok {
		t.Fatalf("GenerationInfo has no %q; keys present: %v",
			base.GenerationInfoRequestID, generationInfoKeys(resp))
	}
	if got != "req_011CQtest" {
		t.Errorf("request id = %v, want req_011CQtest", got)
	}
}

// The streaming path is a separate SDK call with its own options plumbing, so
// it is separately capable of dropping the header. Managed analysis streams —
// the adapter falls back to streaming for any call whose max_tokens implies a
// long run — which makes this the path that matters most in production.
func TestGenerateStreamingReportsTheRequestID(t *testing.T) {
	tr := &requestIDTransport{requestID: "req_011CQstream", streaming: true}

	req := userReq()
	req.StreamingFunc = func(context.Context, []byte) error { return nil }

	resp, err := requestIDModel(t, tr).Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	got, ok := resp.GenerationInfo[base.GenerationInfoRequestID]
	if !ok {
		t.Fatalf("streaming response has no %q; keys present: %v",
			base.GenerationInfoRequestID, generationInfoKeys(resp))
	}
	if got != "req_011CQstream" {
		t.Errorf("request id = %v, want req_011CQstream", got)
	}
}

// A response without the header must omit the key rather than record an empty
// one. Downstream this id is correlated against provider records, where ""
// would be indistinguishable from a call that genuinely reported an id and
// would be looked up fruitlessly.
func TestGenerateOmitsAnAbsentRequestID(t *testing.T) {
	tr := &requestIDTransport{}
	resp, err := requestIDModel(t, tr).Generate(context.Background(), userReq())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if got, ok := resp.GenerationInfo[base.GenerationInfoRequestID]; ok {
		t.Errorf("absent header recorded as %q; the key should be omitted", got)
	}

	// And the rest of GenerationInfo still arrives, so omission is scoped to
	// the missing field rather than a swallowed response.
	if _, ok := resp.GenerationInfo["InputTokens"]; !ok {
		t.Error("token counts went missing alongside the absent request id")
	}
}

// nullBodyTransport returns a 200 whose JSON body is `null`, which the SDK
// decodes into a nil *Message with no error — the shape the two `message !=
// nil` guards in Generate already anticipate.
type nullBodyTransport struct{ requestID string }

func (tr *nullBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	header := http.Header{"Content-Type": []string{"application/json"}}
	if tr.requestID != "" {
		header.Set("request-id", tr.requestID)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     header,
		Body:       io.NopCloser(bytes.NewBufferString("null")),
		Request:    req,
	}, nil
}

// A response carrying no message must not panic, and must still report the id.
//
// responseFromAnthropic returns a zero ModelCallResponse for a nil message,
// whose GenerationInfo is nil rather than empty — so writing a key into it
// panics, taking down an unrelated caller's request. Generate guards
// `message != nil` twice for this case, so the path is treated as reachable by
// code that predates request-id reporting.
func TestGenerateSurvivesAResponseWithNoMessage(t *testing.T) {
	tr := &nullBodyTransport{requestID: "req_011CQnull"}

	resp, err := requestIDModel(t, tr).Generate(context.Background(), userReq())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if resp == nil {
		t.Fatal("nil response")
	}
	if resp.GenerationInfo == nil {
		t.Fatal("GenerationInfo is nil; a returned map must be writable by the next caller too")
	}
	if got := resp.GenerationInfo[base.GenerationInfoRequestID]; got != "req_011CQnull" {
		t.Errorf("request id = %v, want req_011CQnull", got)
	}
}

// recordingTracer captures span attributes so a test can assert what an
// investigator would find on a failed call.
type recordingTracer struct{ span *recordingSpan }

func (t *recordingTracer) StartSpan(ctx context.Context, _ string) (context.Context, hippo.Span) {
	return ctx, t.span
}

type recordingSpan struct{ attrs map[string]any }

func (s *recordingSpan) End() {}
func (s *recordingSpan) SetAttributes(attrs ...hippo.Attribute) {
	for _, a := range attrs {
		s.attrs[a.Key] = a.Value
	}
}
func (s *recordingSpan) AddEvent(string, ...hippo.Attribute) {}
func (s *recordingSpan) RecordError(error)                   {}

// brokenStreamTransport answers with headers and then fails the body read: the
// response arrived, the call did not finish.
type brokenStreamTransport struct{ requestID string }

func (tr *brokenStreamTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	header := http.Header{"Content-Type": []string{"text/event-stream"}}
	header.Set("request-id", tr.requestID)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     header,
		Body:       io.NopCloser(iotest.ErrReader(errors.New("connection reset by peer"))),
		Request:    req,
	}, nil
}

// A call that fails *after* its response arrived still reports the id.
//
// This is the gap between the two obvious cases. Success puts the id in
// GenerationInfo; an API error carries it on sdk.Error. A mid-stream
// connection drop is neither — it is a transport error with no RequestID, and
// Generate returns before GenerationInfo is built — so the id would be
// received and then discarded.
//
// It is also the case most worth correlating: a managed-analysis run that
// streams for twenty minutes, spends tokens, and dies.
func TestGenerateReportsTheRequestIDWhenTheCallFailsAfterTheResponse(t *testing.T) {
	span := &recordingSpan{attrs: map[string]any{}}
	provider := anthropic.NewProvider(
		staticKeyProvider{key: "test-key"},
		anthropic.WithTracer(&recordingTracer{span: span}),
		anthropic.WithRequestOptions(option.WithHTTPClient(&http.Client{
			Transport: &brokenStreamTransport{requestID: "req_011CQdrop"},
		})),
	)
	model, err := provider.Model("requestid_test", hippo.AnthropicClaudeHaiku45)
	if err != nil {
		t.Fatalf("build model: %v", err)
	}

	req := userReq()
	req.StreamingFunc = func(context.Context, []byte) error { return nil }

	_, err = model.Generate(context.Background(), req)
	if err == nil {
		t.Fatal("expected the broken stream to fail the call")
	}

	// The gap, asserted rather than assumed: this failure is a transport error,
	// so the error route carries no RequestID and the span is the only one left.
	// If the SDK ever starts wrapping transport failures as sdk.Error, this
	// fails and the reasoning above needs revisiting.
	var apiErr *sdk.Error
	if errors.As(err, &apiErr) {
		t.Fatalf("expected a transport error; got an sdk.Error carrying RequestID %q, "+
			"which would make the span a second route rather than the only one", apiErr.RequestID)
	}

	if got := span.attrs["llm.request_id"]; got != "req_011CQdrop" {
		t.Errorf("llm.request_id = %v, want req_011CQdrop — the id arrived and was discarded", got)
	}
}

func generationInfoKeys(resp *base.ModelCallResponse) []string {
	keys := make([]string, 0, len(resp.GenerationInfo))
	for k := range resp.GenerationInfo {
		keys = append(keys, k)
	}
	return keys
}
