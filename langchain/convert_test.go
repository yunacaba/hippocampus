package langchain

import (
	"testing"

	"github.com/tmc/langchaingo/llms"

	"github.com/yunacaba/hippocampus/base"
)

func TestMessagesToLangchain(t *testing.T) {
	messages := []base.Message{
		{Role: base.RoleSystem, Parts: []base.ContentPart{base.TextPart{Text: "sys"}}},
		{Role: base.RoleUser, Parts: []base.ContentPart{
			base.TextPart{Text: "hi"},
			base.ImagePart{URL: "https://x/y.png", Detail: "high"},
		}},
		{Role: base.RoleAssistant, Parts: []base.ContentPart{
			base.ToolCallPart{ToolCallID: "1", Name: "search", Arguments: `{"q":"go"}`},
		}},
		{Role: base.RoleTool, Parts: []base.ContentPart{
			base.ToolResultPart{ToolCallID: "1", Name: "search", Content: "ok"},
		}},
	}

	got := messagesToLangchain(messages)
	if len(got) != 4 {
		t.Fatalf("want 4 messages, got %d", len(got))
	}

	if got[0].Role != llms.ChatMessageTypeSystem {
		t.Errorf("system role mismatch: %v", got[0].Role)
	}
	if got[1].Role != llms.ChatMessageTypeHuman {
		t.Errorf("user role should map to Human, got %v", got[1].Role)
	}
	if got[2].Role != llms.ChatMessageTypeAI {
		t.Errorf("assistant role should map to AI, got %v", got[2].Role)
	}
	if got[3].Role != llms.ChatMessageTypeTool {
		t.Errorf("tool role mismatch: %v", got[3].Role)
	}

	img, ok := got[1].Parts[1].(llms.ImageURLContent)
	if !ok || img.URL != "https://x/y.png" || img.Detail != "high" {
		t.Errorf("image part mismatch: %#v", got[1].Parts[1])
	}

	tc, ok := got[2].Parts[0].(llms.ToolCall)
	if !ok || tc.ID != "1" || tc.FunctionCall == nil || tc.FunctionCall.Name != "search" {
		t.Errorf("tool call part mismatch: %#v", got[2].Parts[0])
	}

	tr, ok := got[3].Parts[0].(llms.ToolCallResponse)
	if !ok || tr.ToolCallID != "1" || tr.Content != "ok" {
		t.Errorf("tool result part mismatch: %#v", got[3].Parts[0])
	}
}

func TestOptionsToLangchain(t *testing.T) {
	co := base.ResolveCallOptions([]base.CallOption{
		base.WithTemperature(0.0),
		base.WithJSONMode(),
		base.WithTools([]base.ToolSpec{{Name: "t", Description: "d", Schema: map[string]any{"type": "object"}}}),
	})

	// Apply to a langchaingo CallOptions to verify the options take effect.
	opts := optionsToLangchain(co, nil)
	resolved := &llms.CallOptions{}
	for _, o := range opts {
		o(resolved)
	}

	if resolved.Temperature != 0.0 {
		t.Errorf("temperature: want 0.0, got %v", resolved.Temperature)
	}
	if !resolved.JSONMode {
		t.Error("JSONMode: want true")
	}
	if len(resolved.Tools) != 1 || resolved.Tools[0].Function.Name != "t" {
		t.Errorf("tools: unexpected %#v", resolved.Tools)
	}
}

func TestOptionsToLangchain_ToolChoice(t *testing.T) {
	// Standard mode passes through as a string.
	mode := &llms.CallOptions{}
	for _, o := range optionsToLangchain(base.ResolveCallOptions([]base.CallOption{base.WithToolChoice("required")}), nil) {
		o(mode)
	}
	if mode.ToolChoice != "required" {
		t.Errorf("tool choice mode: want \"required\", got %#v", mode.ToolChoice)
	}

	// A specific tool name becomes a function reference.
	named := &llms.CallOptions{}
	for _, o := range optionsToLangchain(base.ResolveCallOptions([]base.CallOption{base.WithToolChoice("my_tool")}), nil) {
		o(named)
	}
	tc, ok := named.ToolChoice.(llms.ToolChoice)
	if !ok || tc.Function == nil || tc.Function.Name != "my_tool" {
		t.Errorf("named tool choice: unexpected %#v", named.ToolChoice)
	}
}

func TestResponseFromLangchain(t *testing.T) {
	completion := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content:    "hello",
				StopReason: "stop",
				FuncCall:   &llms.FunctionCall{Name: "f", Arguments: `{}`},
				ToolCalls: []llms.ToolCall{
					{ID: "1", FunctionCall: &llms.FunctionCall{Name: "search", Arguments: `{"q":"x"}`}},
				},
			},
		},
	}

	got := responseFromLangchain(completion)
	if got.Content != "hello" || got.StopReason != "stop" {
		t.Errorf("content/stop mismatch: %#v", got)
	}
	if len(got.ToolCalls) != 1 || got.ToolCalls[0].ToolCallID != "1" || got.ToolCalls[0].FunctionCall.Name != "search" {
		t.Errorf("tool calls mismatch: %#v", got.ToolCalls)
	}
}

func TestResponseFromLangchainEmpty(t *testing.T) {
	if got := responseFromLangchain(nil); got == nil {
		t.Fatal("want non-nil response for nil completion")
	}
	if got := responseFromLangchain(&llms.ContentResponse{}); len(got.ToolCalls) != 0 {
		t.Errorf("want empty tool calls, got %#v", got.ToolCalls)
	}
}
