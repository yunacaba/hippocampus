package agenttest

import (
	"context"
	"fmt"
	"sync"

	"github.com/yunacaba/hippocampus/base"
)

// MockAgentObserver records observer callbacks. The agent framework may invoke
// observer methods concurrently (parallel tool execution), so all mutations are
// guarded by a mutex. Read the exported fields only after Execute returns.
type MockAgentObserver[TO any] struct {
	mu                sync.Mutex
	CallSequence      []string
	LLMResponses      []*TO
	LLMErrors         []error
	ToolCalls         []base.ModelToolCall
	ToolCallResponses []base.ModelToolCallResponse
	ToolCallErrors    []error
}

func NewMockAgentObserver[TO any]() *MockAgentObserver[TO] {
	return &MockAgentObserver[TO]{
		CallSequence:      []string{},
		LLMResponses:      []*TO{},
		LLMErrors:         []error{},
		ToolCalls:         []base.ModelToolCall{},
		ToolCallResponses: []base.ModelToolCallResponse{},
		ToolCallErrors:    []error{},
	}
}

func (m *MockAgentObserver[TO]) OnLLMCall(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CallSequence = append(m.CallSequence, "OnLLMCall")
}

func (m *MockAgentObserver[TO]) OnLLMResponse(ctx context.Context, llmResponse TO) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CallSequence = append(m.CallSequence, "OnLLMResponse")
	m.LLMResponses = append(m.LLMResponses, &llmResponse)
}

func (m *MockAgentObserver[TO]) OnLLMError(ctx context.Context, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CallSequence = append(m.CallSequence, "OnLLMError")
	m.LLMErrors = append(m.LLMErrors, err)
}

func (m *MockAgentObserver[TO]) OnToolCall(ctx context.Context, toolCall base.ModelToolCall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CallSequence = append(
		m.CallSequence,
		fmt.Sprintf(
			"OnToolCall: %s ID: %s",
			toolCall.FunctionCall.Name,
			toolCall.ToolCallID,
		),
	)
	m.ToolCalls = append(m.ToolCalls, toolCall)
}

func (m *MockAgentObserver[TO]) OnToolCallResponse(ctx context.Context, toolCallResponse base.ModelToolCallResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CallSequence = append(
		m.CallSequence,
		fmt.Sprintf(
			"OnToolCallResponse: %s ID: %s",
			toolCallResponse.Name,
			toolCallResponse.ToolCallID,
		),
	)
	m.ToolCallResponses = append(m.ToolCallResponses, toolCallResponse)
}

func (m *MockAgentObserver[TO]) OnToolCallError(ctx context.Context, toolCallID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CallSequence = append(
		m.CallSequence,
		fmt.Sprintf("OnToolCallError: %s ID: %s", toolCallID, err.Error()),
	)
	m.ToolCallErrors = append(m.ToolCallErrors, err)
}
