package openai_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/openai/openai-go/v2/option"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/base"
	"github.com/yunacaba/hippocampus/openai"
)

// staticKeyProvider returns a fixed API key for any vendor.
type staticKeyProvider struct{ key string }

func (s staticKeyProvider) APIKey(context.Context, base.LLMVendor) (string, error) {
	return s.key, nil
}

// capturingTransport records the outbound request body and returns a canned
// chat completion response without hitting the network.
type capturingTransport struct {
	body string
}

func (c *capturingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		c.body = string(b)
	}
	const resp = `{"id":"x","object":"chat.completion","created":0,"model":"gpt-4o",` +
		`"choices":[{"index":0,"message":{"role":"assistant","content":"hi","refusal":""},"finish_reason":"stop"}],` +
		`"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(resp)),
		Request:    req,
	}, nil
}

// TestUserAttribution verifies that WithUserID populates the outbound request's
// "user" field.
func TestUserAttribution(t *testing.T) {
	transport := &capturingTransport{}
	httpClient := &http.Client{Transport: transport}

	provider := openai.NewProvider(
		staticKeyProvider{key: "test-key"},
		openai.WithRequestOptions(option.WithHTTPClient(httpClient)),
	)

	model, err := provider.Model("attribution_test", hippo.OpenAIGPT4O)
	if err != nil {
		t.Fatalf("build model: %v", err)
	}

	ctx := hippo.WithUserID(context.Background(), "carmen-user-abc-123")
	_, err = model.Generate(ctx, base.ModelCallRequest{
		Messages: []base.Message{
			{Role: base.RoleUser, Parts: []base.ContentPart{base.TextPart{Text: "hello"}}},
		},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if !strings.Contains(transport.body, `"user":"carmen-user-abc-123"`) {
		t.Errorf("outbound request body missing user attribution.\nbody: %s", transport.body)
	}
}

// TestNoUserAttributionWhenUnset verifies the "user" field is omitted when no
// user ID is set on the context.
func TestNoUserAttributionWhenUnset(t *testing.T) {
	transport := &capturingTransport{}
	httpClient := &http.Client{Transport: transport}

	provider := openai.NewProvider(
		staticKeyProvider{key: "test-key"},
		openai.WithRequestOptions(option.WithHTTPClient(httpClient)),
	)
	model, err := provider.Model("attribution_test", hippo.OpenAIGPT4O)
	if err != nil {
		t.Fatalf("build model: %v", err)
	}

	_, err = model.Generate(context.Background(), base.ModelCallRequest{
		Messages: []base.Message{
			{Role: base.RoleUser, Parts: []base.ContentPart{base.TextPart{Text: "hello"}}},
		},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if strings.Contains(transport.body, `"user":`) {
		t.Errorf("outbound request should not contain a user field.\nbody: %s", transport.body)
	}
}
