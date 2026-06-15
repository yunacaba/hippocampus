package anthropic_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/option"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/anthropic"
	"github.com/yunacaba/hippocampus/base"
)

// streamingProbeTransport records the outbound body (to confirm a streaming
// request was sent) and replies with a minimal Anthropic SSE stream so the
// adapter's accumulate path can build a full message.
type streamingProbeTransport struct {
	body     string
	streamed bool
}

func (c *streamingProbeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		c.body = string(b)
	}
	c.streamed = strings.Contains(c.body, `"stream":true`)

	const sse = "event: message_start\n" +
		`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-haiku-4-5-20251001","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":5,"output_tokens":0}}}` + "\n\n" +
		"event: content_block_start\n" +
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n\n" +
		"event: content_block_delta\n" +
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"streamed-ok"}}` + "\n\n" +
		"event: content_block_stop\n" +
		`data: {"type":"content_block_stop","index":0}` + "\n\n" +
		"event: message_delta\n" +
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":3}}` + "\n\n" +
		"event: message_stop\n" +
		`data: {"type":"message_stop"}` + "\n\n"

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(bytes.NewBufferString(sse)),
		Request:    req,
	}, nil
}

// TestLargeMaxTokensFallsBackToStreaming verifies that a non-streaming Generate
// (no StreamingFunc) with a max_tokens large enough that the SDK would reject a
// plain Messages.New ("streaming is required for operations that may take longer
// than 10 minutes") instead streams-and-accumulates, returning the full result.
func TestLargeMaxTokensFallsBackToStreaming(t *testing.T) {
	transport := &streamingProbeTransport{}
	provider := anthropic.NewProvider(
		staticKeyProvider{key: "test-key"},
		anthropic.WithRequestOptions(option.WithHTTPClient(&http.Client{Transport: transport})),
	)
	model, err := provider.Model("streaming_test", hippo.AnthropicClaudeHaiku45)
	if err != nil {
		t.Fatalf("build model: %v", err)
	}

	req := base.ModelCallRequest{
		Messages: []base.Message{
			{Role: base.RoleUser, Parts: []base.ContentPart{base.TextPart{Text: "hi"}}},
		},
		// 64k exceeds the SDK's ~21k non-streaming budget (10min @ 128k tok/hr).
		Options: []base.CallOption{base.WithMaxTokens(64000)},
		// No StreamingFunc: the caller doesn't want deltas, but the adapter must
		// still stream internally to avoid the non-streaming guard.
	}

	resp, err := model.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("generate must stream for large max_tokens, not reject: %v", err)
	}
	if !transport.streamed {
		t.Errorf("expected a streaming request for large max_tokens; body: %s", transport.body)
	}
	if resp.Content != "streamed-ok" {
		t.Errorf("accumulated content mismatch: %q", resp.Content)
	}
}

// TestSmallMaxTokensStaysNonStreaming guards the inverse: an ordinary
// max_tokens must NOT force streaming (keeps latency/behavior unchanged).
func TestSmallMaxTokensStaysNonStreaming(t *testing.T) {
	transport := &streamingProbeTransport{}
	provider := anthropic.NewProvider(
		staticKeyProvider{key: "test-key"},
		anthropic.WithRequestOptions(option.WithHTTPClient(&http.Client{Transport: transport})),
	)
	model, err := provider.Model("streaming_test", hippo.AnthropicClaudeHaiku45)
	if err != nil {
		t.Fatalf("build model: %v", err)
	}

	req := base.ModelCallRequest{
		Messages: []base.Message{
			{Role: base.RoleUser, Parts: []base.ContentPart{base.TextPart{Text: "hi"}}},
		},
		Options: []base.CallOption{base.WithMaxTokens(4096)},
	}

	// The probe transport only serves SSE; a non-streaming request would get an
	// SSE body the JSON decoder rejects. We only care that the request was NOT
	// marked streaming.
	_, _ = model.Generate(context.Background(), req)
	if transport.streamed {
		t.Errorf("small max_tokens must not force streaming; body: %s", transport.body)
	}
}

// cacheTokenTransport replies with an SSE stream whose message_start carries
// prompt-cache usage (Anthropic reports cache_read/cache_creation in
// message_start; only output_tokens arrives via message_delta).
type cacheTokenTransport struct{}

func (cacheTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	const sse = "event: message_start\n" +
		`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-haiku-4-5-20251001","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":120,"output_tokens":0,"cache_read_input_tokens":800,"cache_creation_input_tokens":50}}}` + "\n\n" +
		"event: content_block_start\n" +
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n\n" +
		"event: content_block_delta\n" +
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok"}}` + "\n\n" +
		"event: content_block_stop\n" +
		`data: {"type":"content_block_stop","index":0}` + "\n\n" +
		"event: message_delta\n" +
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":40}}` + "\n\n" +
		"event: message_stop\n" +
		`data: {"type":"message_stop"}` + "\n\n"
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(bytes.NewBufferString(sse)),
		Request:    req,
	}, nil
}

// TestGenerateSurfacesCacheTokensInMetrics asserts the streaming-accumulation
// path carries Anthropic's cache token counts onto resp.Metrics — the surface
// downstream consumers read.
func TestGenerateSurfacesCacheTokensInMetrics(t *testing.T) {
	provider := anthropic.NewProvider(
		staticKeyProvider{key: "test-key"},
		anthropic.WithRequestOptions(option.WithHTTPClient(&http.Client{Transport: cacheTokenTransport{}})),
	)
	model, err := provider.Model("cache_test", hippo.AnthropicClaudeHaiku45)
	if err != nil {
		t.Fatalf("build model: %v", err)
	}

	// Large max_tokens forces the internal streaming path.
	resp, err := model.Generate(context.Background(), base.ModelCallRequest{
		Messages: []base.Message{{Role: base.RoleUser, Parts: []base.ContentPart{base.TextPart{Text: "hi"}}}},
		Options:  []base.CallOption{base.WithMaxTokens(64000)},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if resp.Metrics == nil {
		t.Fatal("resp.Metrics is nil")
	}
	if resp.Metrics.InputTokens != 120 {
		t.Errorf("InputTokens = %d, want 120", resp.Metrics.InputTokens)
	}
	if resp.Metrics.OutputTokens != 40 {
		t.Errorf("OutputTokens = %d, want 40", resp.Metrics.OutputTokens)
	}
	if resp.Metrics.CacheReadInputTokens != 800 {
		t.Errorf("CacheReadInputTokens = %d, want 800", resp.Metrics.CacheReadInputTokens)
	}
	if resp.Metrics.CacheCreationInputTokens != 50 {
		t.Errorf("CacheCreationInputTokens = %d, want 50", resp.Metrics.CacheCreationInputTokens)
	}
}
