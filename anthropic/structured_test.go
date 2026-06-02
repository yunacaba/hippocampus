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

// toolUseTransport returns a canned tool_use response (as a forced
// structured-output tool would) and records the outbound body.
type toolUseTransport struct{ body string }

func (c *toolUseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		c.body = string(b)
	}
	const resp = `{"id":"msg_x","type":"message","role":"assistant","model":"claude-haiku-4-5-20251001",` +
		`"content":[{"type":"tool_use","id":"tu_1","name":"respond","input":{"answer":"42"}}],` +
		`"stop_reason":"tool_use","stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1}}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(resp)),
		Request:    req,
	}, nil
}

// TestStructuredOutput_LiftsToolInputIntoContent verifies that when a response
// schema forces the synthetic output tool, the tool's input is surfaced as the
// response Content (not as a tool call to execute).
func TestStructuredOutput_LiftsToolInputIntoContent(t *testing.T) {
	transport := &toolUseTransport{}
	provider := anthropic.NewProvider(
		staticKeyProvider{key: "test-key"},
		anthropic.WithRequestOptions(option.WithHTTPClient(&http.Client{Transport: transport})),
	)
	model, err := provider.Model("structured_test", hippo.AnthropicClaudeHaiku45)
	if err != nil {
		t.Fatalf("build model: %v", err)
	}

	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{"answer": map[string]any{"type": "string"}},
	}
	req := base.ModelCallRequest{
		Messages: []base.Message{
			{Role: base.RoleUser, Parts: []base.ContentPart{base.TextPart{Text: "what is 6*7?"}}},
		},
		Options: []base.CallOption{base.WithResponseSchema("respond", schema)},
	}

	resp, err := model.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// The forced tool was offered + selected in the outbound request.
	if !strings.Contains(transport.body, `"tool_choice":{`) || !strings.Contains(transport.body, `"name":"respond"`) {
		t.Errorf("outbound request did not force the output tool: %s", transport.body)
	}

	// The tool input is surfaced as Content, and is not left as a tool call.
	if !strings.Contains(resp.Content, `"answer":"42"`) {
		t.Errorf("tool input not lifted into Content: %q", resp.Content)
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("structured output should not surface tool calls, got %d", len(resp.ToolCalls))
	}
}
