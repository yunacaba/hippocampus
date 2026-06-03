//go:build llm

// Package langchain integration tests exercise a real Google AI call and a real
// local Ollama call. They are gated behind the `llm` build tag:
//
//	GOOGLE_AI_API_KEY=... go test -tags=llm ./langchain/...   # Google AI
//	go test -tags=llm ./langchain/...                         # Ollama (auto-skips if unreachable)
//
// The Ollama test targets OLLAMA_TEST_MODEL (default "gemma4") on the local
// server and skips automatically when no Ollama server is reachable.
package langchain_test

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/langchain"
)

// ollamaBaseURL is the server the test probes/targets: OLLAMA_HOST if set
// (normalized to a URL), else the conventional default. This mirrors how the
// provider (with serverURL "") resolves the host.
func ollamaBaseURL() string {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		return langchain.OllamaServerURL
	}
	if !strings.Contains(host, "://") {
		host = "http://" + host
	}
	return host
}

// ollamaReachable reports whether the Ollama server answers within a short
// timeout, so the integration test can skip cleanly when one isn't running.
func ollamaReachable() bool {
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Get(ollamaBaseURL() + "/api/tags")
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func TestOllamaGenerateEndToEnd(t *testing.T) {
	if !ollamaReachable() {
		t.Skipf("no Ollama server reachable at %s", ollamaBaseURL())
	}
	model := os.Getenv("OLLAMA_TEST_MODEL")
	if model == "" {
		model = "gemma4"
	}

	type req struct {
		Topic string `json:"topic"`
	}
	type resp struct {
		Fact string `json:"fact"`
	}

	provider := langchain.NewOllamaProvider("")
	agent, err := hippo.NewAgentWithTemplateText(
		"Reply with one short JSON object {\"fact\": \"...\"} about {{.topic}}.",
		&req{},
		&resp{Fact: "..."},
	).
		SetName("ollama_fact").
		SetModel(provider, hippo.LLMType(model)).
		Build()
	if err != nil {
		t.Fatalf("build agent: %v", err)
	}

	out, err := agent.Execute(context.Background(), &req{Topic: "the moon"}, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.Fact == "" {
		t.Error("expected a non-empty fact")
	}
}

func TestGoogleAIToolCallEndToEnd(t *testing.T) {
	if os.Getenv("GOOGLE_AI_API_KEY") == "" {
		t.Skip("GOOGLE_AI_API_KEY not set")
	}

	type weatherArgs struct {
		City string `json:"city"`
	}
	type weatherResult struct {
		TempC int `json:"temp_c"`
	}

	called := false
	weatherTool := hippo.NewTool(
		"get_weather",
		"Get the current temperature for a city in Celsius.",
		func(ctx context.Context, in *weatherArgs, _ string) (*weatherResult, error) {
			called = true
			return &weatherResult{TempC: 21}, nil
		},
		&weatherArgs{},
		&weatherResult{},
	)

	type req struct {
		Question string `json:"question"`
	}
	type resp struct {
		Answer string `json:"answer"`
	}

	provider := langchain.NewProvider(hippo.EnvKeyProvider{})
	agent, err := hippo.NewAgentWithTemplateText(
		"Answer the user's question. Use tools when needed.\nQuestion: {{.question}}",
		&req{},
		&resp{Answer: "..."},
	).
		SetName("weather_agent").
		SetModel(provider, hippo.GoogleAIGemini25Flash).
		AddTool(weatherTool).
		Build()
	if err != nil {
		t.Fatalf("build agent: %v", err)
	}

	out, err := agent.Execute(context.Background(), &req{Question: "What is the temperature in Paris?"}, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !called {
		t.Error("expected the weather tool to be called")
	}
	if out.Answer == "" {
		t.Error("expected a non-empty answer")
	}
}
