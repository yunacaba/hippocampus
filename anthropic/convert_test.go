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
