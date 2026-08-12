package anthropic_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

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

func requestIDModel(t *testing.T, tr *requestIDTransport) base.Model {
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

func generationInfoKeys(resp *base.ModelCallResponse) []string {
	keys := make([]string, 0, len(resp.GenerationInfo))
	for k := range resp.GenerationInfo {
		keys = append(keys, k)
	}
	return keys
}
