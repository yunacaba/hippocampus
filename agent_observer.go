package hippocampus

import "context"

// AgentObserver is an interface for observing the agent's execution.
// Mostly used to save the agent's execution to repository.
// Executors should save messages using this interface, and not wait
// for the result of `Execute` calls, as the latter may miss interleaved
// messages.
type AgentObserver[O any] interface {
	ToolObserver

	// Occurs at the start of a new LLM call.
	OnLLMCall(ctx context.Context)

	// Occurs when the LLM call is complete.
	OnLLMResponse(ctx context.Context, llmResponse O)

	// Occurs when an error occurs in the LLM call.
	OnLLMError(ctx context.Context, err error)
}

type noOpAgentObserver[O any] struct{}

func (o *noOpAgentObserver[O]) OnLLMCall(ctx context.Context) {}

func (o *noOpAgentObserver[O]) OnLLMResponse(ctx context.Context, llmResponse O) {}

func (o *noOpAgentObserver[O]) OnLLMError(ctx context.Context, err error) {}

func (o *noOpAgentObserver[O]) OnToolCall(ctx context.Context, toolCall ModelToolCall) {}

func (o *noOpAgentObserver[O]) OnToolCallResponse(ctx context.Context, toolCallResponse ModelToolCallResponse) {
}

func (o *noOpAgentObserver[O]) OnToolCallError(ctx context.Context, toolCallID string, err error) {}

func newNoOpAgentObserver[O any]() AgentObserver[O] {
	return &noOpAgentObserver[O]{}
}
