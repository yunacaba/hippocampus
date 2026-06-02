//go:build llm

// Package openai integration tests exercise a real OpenAI call. They are gated
// behind the `llm` build tag and require OPENAI_API_KEY:
//
//	OPENAI_API_KEY=... go test -tags=llm ./openai/...
package openai_test

import (
	"context"
	"os"
	"testing"

	"github.com/openai/openai-go/v2/option"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/openai"
)

type capitalRequest struct {
	Question string `json:"question"`
}

type capitalAnswer struct {
	Capital string `json:"capital"`
	Country string `json:"country"`
}

// TestOpenAIStructuredOutputEndToEnd exercises the default-on structured-output
// path (response_format json_schema) against the real API: a tool-less agent
// with a typed object output.
func TestOpenAIStructuredOutputEndToEnd(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	provider := openai.NewProvider(hippo.EnvKeyProvider{})
	agent, err := hippo.NewAgentWithTemplateText(
		"Answer the question.\nQuestion: {{.question}}",
		&capitalRequest{},
		&capitalAnswer{},
	).
		SetName("capital_agent").
		SetModel(provider, hippo.OpenAIGPT4OMini).
		Build() // structured output is on by default
	if err != nil {
		t.Fatalf("build agent: %v", err)
	}

	out, err := agent.Execute(context.Background(),
		&capitalRequest{Question: "What is the capital of France, and what country is it in?"}, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.Capital == "" || out.Country == "" {
		t.Errorf("expected a populated typed answer, got %#v", out)
	}
}

// TestOpenAICompatibleLocalStructuredOutput runs the same typed-agent flow
// against a local OpenAI-compatible server (e.g. Ollama). Gated on
// OLLAMA_BASE_URL; set OLLAMA_MODEL to choose the model (default qwen2.5):
//
//	OLLAMA_BASE_URL=http://localhost:11434/v1 OLLAMA_MODEL=qwen2.5 \
//	    go test -tags=llm -run LocalStructuredOutput ./openai/...
//
// It asserts only that a valid typed result comes back — whether the server
// enforces the schema (recent Ollama/llama.cpp constrain via grammar) or the
// tolerant jsonx parser recovers it.
func TestOpenAICompatibleLocalStructuredOutput(t *testing.T) {
	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		t.Skip("OLLAMA_BASE_URL not set")
	}
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "qwen2.5"
	}

	// Local servers need no real key; point the OpenAI client at the base URL.
	provider := openai.NewProvider(
		staticKeyProvider{key: "local"},
		openai.WithRequestOptions(option.WithBaseURL(baseURL)),
	)
	agent, err := hippo.NewAgentWithTemplateText(
		"Answer the question.\nQuestion: {{.question}}",
		&capitalRequest{},
		&capitalAnswer{},
	).
		SetName("local_capital_agent").
		SetModel(provider, hippo.LLMType(model)). // arbitrary local model name
		Build()
	if err != nil {
		t.Fatalf("build agent: %v", err)
	}

	out, err := agent.Execute(context.Background(),
		&capitalRequest{Question: "What is the capital of France, and what country is it in?"}, nil)
	if err != nil {
		t.Fatalf("execute against %s (%s): %v", baseURL, model, err)
	}
	if out.Capital == "" || out.Country == "" {
		t.Errorf("expected a populated typed answer from %s, got %#v", model, out)
	}
}

func TestOpenAIToolCallEndToEnd(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY not set")
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

	provider := openai.NewProvider(hippo.EnvKeyProvider{})
	agent, err := hippo.NewAgentWithTemplateText(
		"Answer the user's question. Use tools when needed.\nQuestion: {{.question}}",
		&req{},
		&resp{Answer: "..."},
	).
		SetName("weather_agent").
		SetModel(provider, hippo.OpenAIGPT4OMini).
		AddTool(weatherTool).
		Build()
	if err != nil {
		t.Fatalf("build agent: %v", err)
	}

	ctx := hippo.WithUserID(context.Background(), "integration-test-user")
	out, err := agent.Execute(ctx, &req{Question: "What is the temperature in Paris?"}, nil)
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
