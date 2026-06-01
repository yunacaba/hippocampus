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

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/openai"
)

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
