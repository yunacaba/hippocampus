//go:build llm

// Package anthropic integration tests exercise a real Anthropic call. They are
// gated behind the `llm` build tag and require ANTHROPIC_API_KEY:
//
//	ANTHROPIC_API_KEY=... go test -tags=llm ./anthropic/...
package anthropic_test

import (
	"context"
	"os"
	"testing"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/anthropic"
)

// TestAnthropicStructuredOutputEndToEnd exercises the default-on structured
// output path (forced output tool) against the real API: a tool-less agent with
// a typed object output.
func TestAnthropicStructuredOutputEndToEnd(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}

	type req struct {
		Question string `json:"question"`
	}
	type answer struct {
		Capital string `json:"capital"`
		Country string `json:"country"`
	}

	provider := anthropic.NewProvider(hippo.EnvKeyProvider{})
	agent, err := hippo.NewAgentWithTemplateText(
		"Answer the question.\nQuestion: {{.question}}",
		&req{},
		&answer{},
	).
		SetName("capital_agent").
		SetModel(provider, hippo.AnthropicClaudeHaiku45).
		Build() // structured output on by default -> forced output tool (no other tools)
	if err != nil {
		t.Fatalf("build agent: %v", err)
	}

	out, err := agent.Execute(context.Background(),
		&req{Question: "What is the capital of France, and what country is it in?"}, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.Capital == "" || out.Country == "" {
		t.Errorf("expected a populated typed answer, got %#v", out)
	}
}

func TestAnthropicToolCallEndToEnd(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
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

	provider := anthropic.NewProvider(hippo.EnvKeyProvider{})
	agent, err := hippo.NewAgentWithTemplateText(
		"Answer the user's question. Use tools when needed.\nQuestion: {{.question}}",
		&req{},
		&resp{Answer: "..."},
	).
		SetName("weather_agent").
		SetModel(provider, hippo.AnthropicClaudeHaiku45).
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
