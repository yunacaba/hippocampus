package openaicompat_test

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
	"github.com/yunacaba/hippocampus/openaicompat"
)

// capturingTransport records the outbound request URL and returns a canned
// chat completion, so tests run without a server.
type capturingTransport struct {
	url    string
	hasKey bool
}

func (c *capturingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.url = req.URL.String()
	c.hasKey = req.Header.Get("Authorization") != ""
	const resp = `{"id":"x","object":"chat.completion","created":0,"model":"gemma3",` +
		`"choices":[{"index":0,"message":{"role":"assistant","content":"hi","refusal":""},"finish_reason":"stop"}],` +
		`"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(resp)),
		Request:    req,
	}, nil
}

func TestNewProvider_RoutesToBaseURLWithArbitraryModel(t *testing.T) {
	tr := &capturingTransport{}
	provider := openaicompat.NewProvider(
		"http://local.test/v1",
		openaicompat.WithRequestOptions(option.WithHTTPClient(&http.Client{Transport: tr})),
	)

	// An arbitrary local model name needs no predefined constant.
	model, err := provider.Model("local", hippo.LLMType("gemma3"))
	if err != nil {
		t.Fatalf("build model: %v", err)
	}

	_, err = model.Generate(context.Background(), base.ModelCallRequest{
		Messages: []base.Message{{Role: base.RoleUser, Parts: []base.ContentPart{base.TextPart{Text: "hi"}}}},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if !strings.Contains(tr.url, "local.test") {
		t.Errorf("request not routed to base URL: %s", tr.url)
	}
	// A non-empty Authorization header is sent (the placeholder key), so servers
	// that require one still work.
	if !tr.hasKey {
		t.Error("expected an Authorization header (placeholder key) to be sent")
	}
}

func TestNewProvider_ResponseSchemaOffByDefault(t *testing.T) {
	provider := openaicompat.NewProvider("http://local.test/v1")
	model, err := provider.Model("local", hippo.LLMType("gemma3"))
	if err != nil {
		t.Fatalf("build model: %v", err)
	}
	c, ok := model.(hippo.ResponseSchemaCapable)
	if !ok {
		t.Fatal("model should implement ResponseSchemaCapable")
	}
	if c.SupportsResponseSchema() {
		t.Error("response-schema support should be off by default for openaicompat")
	}
}

func TestNewProvider_ResponseSchemaOptIn(t *testing.T) {
	provider := openaicompat.NewProvider("http://local.test/v1", openaicompat.WithResponseSchemaSupport(true))
	model, err := provider.Model("local", hippo.LLMType("qwen2.5"))
	if err != nil {
		t.Fatalf("build model: %v", err)
	}
	if !model.(hippo.ResponseSchemaCapable).SupportsResponseSchema() {
		t.Error("response-schema support should be enabled via WithResponseSchemaSupport(true)")
	}
}

func TestPresets(t *testing.T) {
	if openaicompat.OllamaBaseURL == "" || openaicompat.LMStudioBaseURL == "" {
		t.Fatal("preset base URLs should be set")
	}
	// Presets build without error and accept arbitrary models.
	for _, p := range []base.ModelProvider{openaicompat.Ollama(), openaicompat.LMStudio()} {
		if _, err := p.Model("local", hippo.LLMType("gemma3")); err != nil {
			t.Errorf("preset provider rejected model: %v", err)
		}
	}
}
