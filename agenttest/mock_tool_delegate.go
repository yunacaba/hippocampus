package agenttest

import (
	"context"

	"github.com/yunacaba/hippocampus/base"
)

// MockToolDelegate is a mock implementation of the ToolDelegate interface for testing.
type MockToolDelegate struct {
	RewriteToolCallsFunc func(ctx context.Context, toolCalls []*base.ModelToolCall)

	// Tracking fields for assertions
	RewriteToolCallsCallCount int
	LastToolCalls             []*base.ModelToolCall
}

// RewriteToolCalls implements the ToolDelegate interface.
func (m *MockToolDelegate) RewriteToolCalls(
	ctx context.Context,
	toolCalls []*base.ModelToolCall,
) {
	m.RewriteToolCallsCallCount++
	m.LastToolCalls = make([]*base.ModelToolCall, len(toolCalls))
	copy(m.LastToolCalls, toolCalls)

	if m.RewriteToolCallsFunc != nil {
		m.RewriteToolCallsFunc(ctx, toolCalls)
		return
	}

	// Default behavior: accept all tool calls
	for _, toolCall := range toolCalls {
		toolCall.DelegateAction = base.ToolDelegateActionAccepted
	}
}

// Reset resets the mock state for reuse in tests.
func (m *MockToolDelegate) Reset() {
	m.RewriteToolCallsCallCount = 0
	m.LastToolCalls = nil
}
