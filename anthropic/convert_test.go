package anthropic

import (
	"encoding/json"
	"strings"
	"testing"

	sdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/yunacaba/hippocampus/base"
)

func marshalParams(t *testing.T, opts ...base.CallOption) string {
	t.Helper()
	var params sdk.MessageNewParams
	applyOptions(&params, base.ResolveCallOptions(opts))
	b, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	return string(b)
}

func TestApplyOptions_DefaultMaxTokens(t *testing.T) {
	s := marshalParams(t)
	if !strings.Contains(s, `"max_tokens":4096`) {
		t.Errorf("default max_tokens not applied: %s", s)
	}
}

func TestApplyOptions_StopWords(t *testing.T) {
	s := marshalParams(t, base.WithStopWords([]string{"STOP"}))
	if !strings.Contains(s, `"stop_sequences":["STOP"]`) {
		t.Errorf("stop words not mapped: %s", s)
	}
}

func TestApplyOptions_ToolChoiceModes(t *testing.T) {
	cases := map[string]string{
		"auto":     `"tool_choice":{"type":"auto"}`,
		"required": `"tool_choice":{"type":"any"}`,
		"none":     `"tool_choice":{"type":"none"}`,
	}
	for mode, want := range cases {
		s := marshalParams(t, base.WithToolChoice(mode))
		if !strings.Contains(s, want) {
			t.Errorf("tool_choice %q: want %s in %s", mode, want, s)
		}
	}
}

func TestApplyOptions_NamedToolChoice(t *testing.T) {
	s := marshalParams(t, base.WithToolChoice("my_tool"))
	// JSON key order within the object is not guaranteed; check the pieces.
	if !strings.Contains(s, `"tool_choice":{`) ||
		!strings.Contains(s, `"name":"my_tool"`) ||
		!strings.Contains(s, `"type":"tool"`) {
		t.Errorf("named tool_choice not mapped: %s", s)
	}
}

func TestApplyOptions_JSONModeAddsSystemInstruction(t *testing.T) {
	s := marshalParams(t, base.WithJSONMode())
	if !strings.Contains(s, `"system":`) || !strings.Contains(s, "valid JSON object") {
		t.Errorf("json mode system instruction not added: %s", s)
	}
}

func TestApplyOptions_PromptCachingMarksSystem(t *testing.T) {
	params := sdk.MessageNewParams{System: []sdk.TextBlockParam{{Text: "big reusable system prompt"}}}
	applyOptions(&params, base.ResolveCallOptions([]base.CallOption{base.WithPromptCaching()}))
	b, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"cache_control"`) {
		t.Errorf("expected cache_control on the system prompt: %s", b)
	}
}

func TestApplyOptions_ThinkingEnabled(t *testing.T) {
	s := marshalParams(t, base.WithThinking())
	if !strings.Contains(s, `"thinking"`) || !strings.Contains(s, `"budget_tokens"`) {
		t.Errorf("expected thinking config with a budget: %s", s)
	}
	if !strings.Contains(s, `"type":"enabled"`) {
		t.Errorf("expected thinking type enabled: %s", s)
	}
}

func TestApplyOptions_ThinkingSuppressesTemperature(t *testing.T) {
	// Extended thinking is incompatible with a custom temperature.
	s := marshalParams(t, base.WithTemperature(0.5), base.WithThinking())
	if strings.Contains(s, `"temperature"`) {
		t.Errorf("temperature must be omitted when thinking is enabled: %s", s)
	}
}

func TestApplyOptions_ThinkingBudgetBumpsMaxTokens(t *testing.T) {
	// max_tokens must exceed the thinking budget.
	s := marshalParams(t, base.WithMaxTokens(1000), base.WithThinkingBudget(8000))
	// budget 8000 > maxTokens 1000, so max_tokens should have been raised.
	if strings.Contains(s, `"max_tokens":1000`) {
		t.Errorf("max_tokens should be raised above the thinking budget: %s", s)
	}
}

func TestApplyOptions_ResponseSchemaForcesTool(t *testing.T) {
	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{"answer": map[string]any{"type": "string"}},
	}
	s := marshalParams(t, base.WithResponseSchema("respond", schema))
	// A single tool is offered and forced via tool_choice.
	if !strings.Contains(s, `"tool_choice":{`) || !strings.Contains(s, `"name":"respond"`) {
		t.Errorf("expected forced tool_choice for the output tool: %s", s)
	}
	if !strings.Contains(s, `"input_schema"`) || !strings.Contains(s, `"answer"`) {
		t.Errorf("expected the output tool to carry the schema: %s", s)
	}
}

func TestApplyOptions_ResponseSchemaSkippedWithRealTools(t *testing.T) {
	schema := map[string]any{"type": "object"}
	// With real tools present, forcing the output tool would block them, so the
	// schema enforcement is skipped (tool_choice not forced to "respond").
	s := marshalParams(
		t,
		base.WithTools([]base.ToolSpec{{Name: "search", Description: "d", Schema: map[string]any{"type": "object"}}}),
		base.WithResponseSchema("respond", schema),
	)
	if strings.Contains(s, `"name":"respond"`) {
		t.Errorf("output tool should not be forced when real tools are present: %s", s)
	}
}

func TestResponseFromAnthropic_SurfacesCacheTokens(t *testing.T) {
	msg := &sdk.Message{
		StopReason: "end_turn",
		Content:    []sdk.ContentBlockUnion{{Type: "text", Text: "hi"}},
		Usage: sdk.Usage{
			InputTokens:              120,
			OutputTokens:             40,
			CacheReadInputTokens:     800,
			CacheCreationInputTokens: 50,
		},
	}

	resp := responseFromAnthropic(msg)

	if got := resp.GenerationInfo["InputTokens"]; got != 120 {
		t.Errorf("InputTokens = %v, want 120", got)
	}
	if got := resp.GenerationInfo["CacheReadInputTokens"]; got != 800 {
		t.Errorf("CacheReadInputTokens = %v, want 800", got)
	}
	if got := resp.GenerationInfo["CacheCreationInputTokens"]; got != 50 {
		t.Errorf("CacheCreationInputTokens = %v, want 50", got)
	}
}
