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

type staticKeyProvider struct{ key string }

func (s staticKeyProvider) APIKey(context.Context, base.LLMVendor) (string, error) {
	return s.key, nil
}

// capturingTransport records the outbound request body and returns a canned
// Anthropic message response without hitting the network.
type capturingTransport struct {
	body string
}

func (c *capturingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		c.body = string(b)
	}
	const resp = `{"id":"msg_x","type":"message","role":"assistant","model":"claude-haiku-4-5-20251001",` +
		`"content":[{"type":"text","text":"hi"}],"stop_reason":"end_turn","stop_sequence":null,` +
		`"usage":{"input_tokens":1,"output_tokens":1}}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(resp)),
		Request:    req,
	}, nil
}

func newModel(t *testing.T, transport *capturingTransport) base.Model {
	t.Helper()
	provider := anthropic.NewProvider(
		staticKeyProvider{key: "test-key"},
		anthropic.WithRequestOptions(option.WithHTTPClient(&http.Client{Transport: transport})),
	)
	model, err := provider.Model("attribution_test", hippo.AnthropicClaudeHaiku45)
	if err != nil {
		t.Fatalf("build model: %v", err)
	}
	return model
}

func userReq() base.ModelCallRequest {
	return base.ModelCallRequest{
		Messages: []base.Message{
			{Role: base.RoleUser, Parts: []base.ContentPart{base.TextPart{Text: "hello"}}},
		},
	}
}

// TestUserAttribution verifies WithUserID populates metadata.user_id.
func TestUserAttribution(t *testing.T) {
	transport := &capturingTransport{}
	model := newModel(t, transport)

	ctx := hippo.WithUserID(context.Background(), "carmen-user-abc-123")
	if _, err := model.Generate(ctx, userReq()); err != nil {
		t.Fatalf("generate: %v", err)
	}

	if !strings.Contains(transport.body, `"user_id":"carmen-user-abc-123"`) {
		t.Errorf("outbound request body missing metadata.user_id.\nbody: %s", transport.body)
	}
	if !strings.Contains(transport.body, `"metadata":`) {
		t.Errorf("outbound request body missing metadata object.\nbody: %s", transport.body)
	}
}

// TestNoUserAttributionWhenUnset verifies metadata.user_id is omitted when no
// user ID is set on the context.
func TestNoUserAttributionWhenUnset(t *testing.T) {
	transport := &capturingTransport{}
	model := newModel(t, transport)

	if _, err := model.Generate(context.Background(), userReq()); err != nil {
		t.Fatalf("generate: %v", err)
	}

	if strings.Contains(transport.body, `"user_id":`) {
		t.Errorf("outbound request should not contain user_id.\nbody: %s", transport.body)
	}
}

// TestSystemPromptHoisted verifies that system-role messages are hoisted into
// the top-level system field rather than sent as a message.
func TestSystemPromptHoisted(t *testing.T) {
	transport := &capturingTransport{}
	model := newModel(t, transport)

	req := base.ModelCallRequest{
		Messages: []base.Message{
			{Role: base.RoleSystem, Parts: []base.ContentPart{base.TextPart{Text: "You are a pirate."}}},
			{Role: base.RoleUser, Parts: []base.ContentPart{base.TextPart{Text: "hello"}}},
		},
	}
	if _, err := model.Generate(context.Background(), req); err != nil {
		t.Fatalf("generate: %v", err)
	}

	if !strings.Contains(transport.body, `"system":`) {
		t.Errorf("expected top-level system field.\nbody: %s", transport.body)
	}
	if !strings.Contains(transport.body, "You are a pirate.") {
		t.Errorf("system prompt content missing.\nbody: %s", transport.body)
	}
	// The system text must not appear as a role:"system" message.
	if strings.Contains(transport.body, `"role":"system"`) {
		t.Errorf("system prompt should not be sent as a message role.\nbody: %s", transport.body)
	}
}
