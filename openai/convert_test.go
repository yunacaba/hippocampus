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
