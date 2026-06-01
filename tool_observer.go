package hippocampus

import "context"

type ToolObserver interface {
	OnToolCall(ctx context.Context, toolCall ModelToolCall)
	OnToolCallResponse(ctx context.Context, toolCallResponse ModelToolCallResponse)
	OnToolCallError(ctx context.Context, toolCallID string, err error)
}

type noOpToolObserver struct{}

var _ ToolObserver = &noOpToolObserver{}

func (o *noOpToolObserver) OnToolCall(ctx context.Context, toolCall ModelToolCall) {}

func (o *noOpToolObserver) OnToolCallResponse(ctx context.Context, toolCallResponse ModelToolCallResponse) {
}

func (o *noOpToolObserver) OnToolCallError(ctx context.Context, toolCallID string, err error) {}

func newNoOpToolObserver() ToolObserver {
	return &noOpToolObserver{}
}
