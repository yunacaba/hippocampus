package hippocampus

import (
	"context"
	"testing"

	"github.com/yunacaba/hippocampus/base"
)

// TestExecuteModelToolCalls_EmptyIDsPairCorrectly guards against the tool-call
// pairing bug: when a provider returns multiple tool calls with empty (or
// duplicate) IDs — as Google AI does — each result must still be paired with
// the exact call that produced it, not collapsed by ID.
func TestExecuteModelToolCalls_EmptyIDsPairCorrectly(t *testing.T) {
	type echoIn struct {
		V string `json:"v"`
	}
	type echoOut struct {
		Echo string `json:"echo"`
	}

	echoTool := NewTool(
		"echo",
		"echoes its input",
		func(_ context.Context, in *echoIn, _ string) (*echoOut, error) {
			return &echoOut{Echo: in.V}, nil
		},
		&echoIn{},
		&echoOut{},
	)

	tb, err := newToolbox("test", []AnyTool{echoTool}, nil, nil, NoopTracer{}, false)
	if err != nil {
		t.Fatalf("newToolbox: %v", err)
	}

	// Two calls to the same tool, both with empty IDs and distinct arguments.
	calls := newModelToolCallArray([]base.ModelToolCall{
		{ToolCallID: "", FunctionCall: base.FunctionCall{Name: "echo", Arguments: `{"v":"A"}`}},
		{ToolCallID: "", FunctionCall: base.FunctionCall{Name: "echo", Arguments: `{"v":"B"}`}},
	})

	result := tb.executeModelToolCalls(context.Background(), calls)

	// Expect 4 messages: callA, resultA, callB, resultB — in call order.
	if len(result.pendingMessages) != 4 {
		t.Fatalf("want 4 pending messages, got %d", len(result.pendingMessages))
	}

	callArgs := func(m ModelMessage) string {
		if tc, ok := m.Parts[0].(base.ToolCallPart); ok {
			return tc.Arguments
		}
		t.Fatalf("expected ToolCallPart, got %T", m.Parts[0])
		return ""
	}
	resultContent := func(m ModelMessage) string {
		if tr, ok := m.Parts[0].(base.ToolResultPart); ok {
			return tr.Content
		}
		t.Fatalf("expected ToolResultPart, got %T", m.Parts[0])
		return ""
	}

	// The two tool-call messages must be distinct (the bug made them identical).
	if a, b := callArgs(result.pendingMessages[0]), callArgs(result.pendingMessages[2]); a == b {
		t.Errorf("tool-call messages collapsed to identical args: both %q", a)
	}

	// Each call must be paired with its own result.
	if got := callArgs(result.pendingMessages[0]); got != `{"v":"A"}` {
		t.Errorf("call[0] args = %q, want A", got)
	}
	if got := resultContent(result.pendingMessages[1]); got != `{"echo":"A"}` {
		t.Errorf("result[0] = %q, want echo A", got)
	}
	if got := callArgs(result.pendingMessages[2]); got != `{"v":"B"}` {
		t.Errorf("call[1] args = %q, want B", got)
	}
	if got := resultContent(result.pendingMessages[3]); got != `{"echo":"B"}` {
		t.Errorf("result[1] = %q, want echo B", got)
	}
}
