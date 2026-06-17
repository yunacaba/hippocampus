package base

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestContentPartRoundTrip verifies that a Message containing every ContentPart
// variant survives a JSON marshal/unmarshal cycle unchanged.
func TestContentPartRoundTrip(t *testing.T) {
	parts := []ContentPart{
		TextPart{Text: "hello world"},
		ImagePart{URL: "https://example.com/cat.png", Detail: "high"},
		BinaryPart{MIMEType: "application/pdf", Data: []byte("%PDF-1.7 binary")},
		ToolCallPart{ToolCallID: "call_1", Name: "search", Arguments: `{"q":"go"}`},
		ToolResultPart{ToolCallID: "call_1", Name: "search", Content: "ok", IsError: false},
		ToolResultPart{ToolCallID: "call_2", Name: "search", Content: "boom", IsError: true},
	}

	for _, p := range parts {
		t.Run(p.partType(), func(t *testing.T) {
			original := Message{Role: RoleAssistant, Parts: []ContentPart{p}}

			data, err := json.Marshal(original)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var got Message
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if !reflect.DeepEqual(original, got) {
				t.Fatalf("round-trip mismatch\n  want: %#v\n  got:  %#v\n  json: %s", original, got, data)
			}
		})
	}
}

// TestMultiPartMessageRoundTrip verifies a single message with mixed parts.
func TestMultiPartMessageRoundTrip(t *testing.T) {
	original := Message{
		Role: RoleUser,
		Parts: []ContentPart{
			TextPart{Text: "describe this"},
			ImagePart{URL: "data:image/png;base64,AAAA"},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Message
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(original, got) {
		t.Fatalf("round-trip mismatch\n  want: %#v\n  got:  %#v", original, got)
	}
}

// TestTextPartCacheBreakpointRoundTrip verifies the CacheBreakpoint flag
// survives JSON serialization.
func TestTextPartCacheBreakpointRoundTrip(t *testing.T) {
	original := Message{
		Role: RoleUser,
		Parts: []ContentPart{
			TextPart{Text: "shared node content", CacheBreakpoint: true},
			TextPart{Text: "agent-specific instructions"},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Message
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(original, got) {
		t.Fatalf("round-trip mismatch\n  want: %#v\n  got:  %#v\n  json: %s", original, got, data)
	}
}

// TestUnknownPartTypeRejected ensures decoding fails on an unknown discriminator.
func TestUnknownPartTypeRejected(t *testing.T) {
	var m Message
	err := json.Unmarshal([]byte(`{"role":"user","parts":[{"type":"video","url":"x"}]}`), &m)
	if err == nil {
		t.Fatal("expected error for unknown part type, got nil")
	}
}

// TestResolveCallOptions checks the functional-option resolution.
func TestResolveCallOptions(t *testing.T) {
	co := ResolveCallOptions([]CallOption{
		WithTemperature(0.5),
		WithJSONMode(),
		WithTools([]ToolSpec{{Name: "t", Description: "d", Schema: map[string]any{"type": "object"}}}),
		WithMaxTokens(128),
	})

	if co.Temperature == nil || *co.Temperature != 0.5 {
		t.Errorf("temperature: want 0.5, got %v", co.Temperature)
	}
	if !co.JSONMode {
		t.Error("JSONMode: want true")
	}
	if len(co.Tools) != 1 || co.Tools[0].Name != "t" {
		t.Errorf("tools: unexpected %#v", co.Tools)
	}
	if co.MaxTokens != 128 {
		t.Errorf("max tokens: want 128, got %d", co.MaxTokens)
	}
}
