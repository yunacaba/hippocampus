//go:build llm

// Package openaicompat integration tests run a typed agent against a real local
// OpenAI-compatible server. Gated behind the `llm` build tag and OLLAMA_BASE_URL;
// set OLLAMA_MODEL to choose the model (default qwen2.5):
//
//	OLLAMA_BASE_URL=http://localhost:11434/v1 OLLAMA_MODEL=qwen2.5 \
//	    go test -tags=llm ./openaicompat/...
package openaicompat_test

import (
	"context"
	"os"
	"testing"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/openaicompat"
)

func TestOllamaStructuredOutputEndToEnd(t *testing.T) {
	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		t.Skip("OLLAMA_BASE_URL not set")
	}
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "qwen2.5"
	}

	type req struct {
		Question string `json:"question"`
	}
	type answer struct {
		Capital string `json:"capital"`
		Country string `json:"country"`
	}

	// Use the default (schema support off): the prompt-guided + tolerant-parser
	// path works against any OpenAI-compatible server, so this is a reliable
	// end-to-end smoke test. To exercise response_format enforcement against a
	// server that supports it, add openaicompat.WithResponseSchemaSupport(true).
	provider := openaicompat.NewProvider(baseURL)
	agent, err := hippo.NewAgentWithTemplateText(
		"Answer the question.\nQuestion: {{.question}}",
		&req{},
		&answer{},
	).
		SetName("local_capital_agent").
		SetModel(provider, hippo.LLMType(model)).
		Build()
	if err != nil {
		t.Fatalf("build agent: %v", err)
	}

	out, err := agent.Execute(context.Background(),
		&req{Question: "What is the capital of France, and what country is it in?"}, nil)
	if err != nil {
		t.Fatalf("execute against %s (%s): %v", baseURL, model, err)
	}
	if out.Capital == "" || out.Country == "" {
		t.Errorf("expected a populated typed answer from %s, got %#v", model, out)
	}
}
