package openai

import (
	"encoding/json"
	"strings"
	"testing"

	oai "github.com/openai/openai-go/v2"

	"github.com/yunacaba/hippocampus/base"
)

func marshalParams(t *testing.T, opts ...base.CallOption) string {
	t.Helper()
	var params oai.ChatCompletionNewParams
	applyOptions(&params, base.ResolveCallOptions(opts))
	b, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	return string(b)
}

func TestApplyOptions_StopWords(t *testing.T) {
	s := marshalParams(t, base.WithStopWords([]string{"STOP", "END"}))
	if !strings.Contains(s, `"stop":["STOP","END"]`) {
		t.Errorf("stop words not mapped: %s", s)
	}
}

func TestApplyOptions_ToolChoiceModes(t *testing.T) {
	for _, mode := range []string{"auto", "required", "none"} {
		s := marshalParams(t, base.WithToolChoice(mode))
		if !strings.Contains(s, `"tool_choice":"`+mode+`"`) {
			t.Errorf("tool_choice %q not mapped: %s", mode, s)
		}
	}
}

func TestApplyOptions_NamedToolChoice(t *testing.T) {
	s := marshalParams(t, base.WithToolChoice("my_tool"))
	// JSON key order within the object is not guaranteed; check the pieces.
	if !strings.Contains(s, `"tool_choice":{`) ||
		!strings.Contains(s, `"function":{"name":"my_tool"}`) {
		t.Errorf("named tool_choice not mapped: %s", s)
	}
}

func TestApplyOptions_JSONMode(t *testing.T) {
	s := marshalParams(t, base.WithJSONMode())
	if !strings.Contains(s, `"response_format":{"type":"json_object"}`) {
		t.Errorf("json mode not mapped: %s", s)
	}
}

func TestApplyOptions_ResponseSchema(t *testing.T) {
	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{"answer": map[string]any{"type": "string"}},
	}
	s := marshalParams(t, base.WithResponseSchema("response", schema))
	if !strings.Contains(s, `"type":"json_schema"`) {
		t.Errorf("response_format not json_schema: %s", s)
	}
	if !strings.Contains(s, `"name":"response"`) {
		t.Errorf("schema name missing: %s", s)
	}
	if !strings.Contains(s, `"answer"`) {
		t.Errorf("schema body missing: %s", s)
	}
}

func TestApplyOptions_ResponseSchemaBeatsJSONMode(t *testing.T) {
	schema := map[string]any{"type": "object"}
	// Both set (the agent sets both): the schema must win.
	s := marshalParams(t, base.WithJSONMode(), base.WithResponseSchema("response", schema))
	if !strings.Contains(s, `"type":"json_schema"`) {
		t.Errorf("expected json_schema to take precedence: %s", s)
	}
	if strings.Contains(s, `"type":"json_object"`) {
		t.Errorf("json_object should not be present: %s", s)
	}
}
