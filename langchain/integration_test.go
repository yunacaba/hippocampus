//go:build llm

// Package langchain integration tests exercise a real Google AI call. They are
// gated behind the `llm` build tag and require GOOGLE_AI_API_KEY:
//
//	GOOGLE_AI_API_KEY=... go test -tags=llm ./langchain/...
package langchain_test

import (
	"context"
	"os"
	"testing"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/langchain"
)

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
