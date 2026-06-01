package hippocampus

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/yunacaba/hippocampus/base"
)

// ToolDelegate is called to rewrite tool calls before they are executed.
// Complex planning logic can process these calls to refine the LLM's
// decisions.
type ToolDelegate interface {
	// RewriteToolCalls is called before tools are executed.
	// Tool calls are modified in place.
	RewriteToolCalls(ctx context.Context, toolCalls []*ModelToolCall)
}

type noOpToolDelegate struct{}

var _ ToolDelegate = &noOpToolDelegate{}

func (n *noOpToolDelegate) RewriteToolCalls(
	ctx context.Context,
	toolCalls []*ModelToolCall,
) {
	for _, toolCall := range toolCalls {
		toolCall.DelegateAction = base.ToolDelegateActionAccepted
	}
}

// toolInvoker is the strategy interface for executing individual tool calls.
// This abstraction allows the evaluation framework to inject mocking behavior
// without polluting production code.
type toolInvoker interface {
	invoke(ctx context.Context, toolbox *Toolbox, toolCall ModelToolCall) ModelToolCallResponse
}

// ToolInvoker is exported to allow the evaluation framework to wrap it.
// Production code should not need to reference this directly.
type ToolInvoker interface {
	Invoke(ctx context.Context, toolbox *Toolbox, toolCall ModelToolCall) ModelToolCallResponse
}

// directToolInvoker is the default production implementation that executes tools directly.
type directToolInvoker struct{}

func (d *directToolInvoker) invoke(ctx context.Context, tb *Toolbox, toolCall ModelToolCall) ModelToolCallResponse {
	// Create a timeout context derived from parent to preserve cancellation chain
	toolCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	tb.observer.OnToolCall(ctx, toolCall)

	// Create a span in the original context for tracing
	_, span := tb.tracer.StartSpan(ctx, fmt.Sprintf("%s.Tools.%s", tb.name, toolCall.FunctionCall.Name))
	tb.debugAttributes(
		span,
		StringAttr("tool.name", toolCall.FunctionCall.Name),
		StringAttr("tool.id", toolCall.ToolCallID),
		StringAttr("tool.arguments", toolCall.FunctionCall.Arguments),
	)
	defer span.End()

	startTime := time.Now()

	toolResponse, err := tb.executeToolCallStringResponse(toolCtx, toolCall)
	if err != nil {
		span.RecordError(err)
	}

	tb.debugAttributes(span, StringAttr("tool.result", toolResponse))

	duration := time.Since(startTime)
	tb.debugAttributes(span, IntAttr("tool.duration", int(duration.Milliseconds())))

	result := ModelToolCallResponse{
		ToolCallID: toolCall.ToolCallID,
		Name:       toolCall.FunctionCall.Name,
		Content:    toolResponse,
		Error:      err,
		Metrics: ModelToolCallMetrics{
			ToolName:      toolCall.FunctionCall.Name,
			StartTime:     startTime,
			TotalDuration: duration,
		},
	}
	if result.Error != nil {
		tb.observer.OnToolCallError(ctx, toolCall.ToolCallID, result.Error)
	} else {
		tb.observer.OnToolCallResponse(ctx, result)
	}

	return result
}

// Invoke implements the exported ToolInvoker interface.
func (d *directToolInvoker) Invoke(ctx context.Context, toolbox *Toolbox, toolCall ModelToolCall) ModelToolCallResponse {
	return d.invoke(ctx, toolbox, toolCall)
}

// Toolbox coordinates the execution of tool calls.
type Toolbox struct {
	name           string
	tools          map[string]AnyTool
	observer       ToolObserver
	delegate       ToolDelegate
	invoker        toolInvoker
	tracer         Tracer
	debugToolCalls bool
}

func newToolbox(
	name string,
	tools []AnyTool,
	observer ToolObserver,
	delegate ToolDelegate,
	tracer Tracer,
	debugToolCalls bool,
) (*Toolbox, error) {
	toolsMap := make(map[string]AnyTool)
	for _, tool := range tools {
		toolsMap[tool.Name()] = tool
	}
	if observer == nil {
		observer = newNoOpToolObserver()
	}
	if delegate == nil {
		delegate = &noOpToolDelegate{}
	}
	if tracer == nil {
		tracer = NoopTracer{}
	}
	return &Toolbox{
		name:           name,
		tools:          toolsMap,
		observer:       observer,
		delegate:       delegate,
		invoker:        &directToolInvoker{},
		tracer:         tracer,
		debugToolCalls: debugToolCalls,
	}, nil
}

func (t *Toolbox) Tools() map[string]AnyTool {
	return t.tools
}

type toolboxToolCallResult struct {
	pendingMessages []ModelMessage
	toolResponses   []ModelToolCallResponse
	toolCallMetrics []ModelToolCallMetrics
}

func newToolboxToolCallResult(
	pendingMessages []ModelMessage,
	toolResponses []ModelToolCallResponse,
	toolCallMetrics []ModelToolCallMetrics,
) toolboxToolCallResult {
	return toolboxToolCallResult{
		pendingMessages: pendingMessages,
		toolResponses:   toolResponses,
		toolCallMetrics: toolCallMetrics,
	}
}

// newModelToolCallArray converts the model's surfaced tool calls into a mutable
// slice of pointers with a pending delegate action, ready for the delegate to
// rewrite.
func newModelToolCallArray(toolCalls []ModelToolCall) []*ModelToolCall {
	modelToolCallArray := make([]*ModelToolCall, 0, len(toolCalls))
	for i := range toolCalls {
		tc := toolCalls[i]
		tc.DelegateAction = base.ToolDelegateActionPending
		modelToolCallArray = append(modelToolCallArray, &tc)
	}
	return modelToolCallArray
}

func (t *Toolbox) executeLLMToolCalls(
	ctx context.Context,
	toolCalls []ModelToolCall,
) toolboxToolCallResult {
	modelToolCalls := newModelToolCallArray(toolCalls)
	return t.executeModelToolCalls(ctx, modelToolCalls)
}

func (t *Toolbox) executeModelToolCalls(
	ctx context.Context,
	toolCalls []*ModelToolCall,
) toolboxToolCallResult {
	// Create a new span in the context passed from Execute
	toolsCtx, span := t.tracer.StartSpan(ctx, fmt.Sprintf("%s.Tools", t.name))
	defer span.End()

	// Rewrite tool calls via delegate before execution
	t.delegate.RewriteToolCalls(toolsCtx, toolCalls)
	verifyToolCallsRewriteResult(toolCalls)

	// Execute calls concurrently but collect results positionally, so each
	// response is paired with the exact tool call that produced it. Pairing by
	// ToolCallID would collapse calls that share an ID or have an empty ID
	// (e.g. Google AI never returns tool-call IDs).
	responses := make([]ModelToolCallResponse, len(toolCalls))
	var wg sync.WaitGroup
	for i, toolCall := range toolCalls {
		if toolCall.DelegateAction == base.ToolDelegateActionRejected {
			responses[i] = t.rejectToolCall(toolsCtx, *toolCall)
			continue
		}
		wg.Add(1)
		go func(i int, tc ModelToolCall) {
			defer wg.Done()
			responses[i] = t.executeToolCall(toolsCtx, tc)
		}(i, *toolCall)
	}
	wg.Wait()

	// 2 messages (call + response) per tool call, paired in call order.
	pendingMessages := make([]ModelMessage, 0, 2*len(toolCalls))
	toolResponses := make([]ModelToolCallResponse, 0, len(toolCalls))
	toolCallMetrics := make([]ModelToolCallMetrics, 0, len(toolCalls))
	for i, toolCall := range toolCalls {
		response := responses[i]
		pendingMessages = append(pendingMessages, NewToolCallMessage(*toolCall))
		pendingMessages = append(pendingMessages, NewToolCallResponseMessage(response))
		toolResponses = append(toolResponses, response)
		toolCallMetrics = append(toolCallMetrics, response.Metrics)
	}
	t.debugConsoleLog(
		"Tool calls: %v (%d count)",
		pendingMessages,
		len(toolResponses),
	)

	return newToolboxToolCallResult(pendingMessages, toolResponses, toolCallMetrics)
}

// If any tool calls are left pending, reject them with an internal error.
func verifyToolCallsRewriteResult(toolCalls []*ModelToolCall) {
	for _, toolCall := range toolCalls {
		if toolCall.DelegateAction == base.ToolDelegateActionPending {
			toolCall.RejectWithReason("internal error")
		}
	}
}

func (t *Toolbox) executeToolCall(
	ctx context.Context,
	toolCall ModelToolCall,
) ModelToolCallResponse {
	// Delegate to pluggable invoker strategy
	return t.invoker.invoke(ctx, t, toolCall)
}

// SetToolInvoker replaces the default invoker with a custom implementation.
// This is used by the evaluation framework to inject mocking behavior.
func (t *Toolbox) SetToolInvoker(invoker ToolInvoker) {
	// Create adapter from exported ToolInvoker to internal toolInvoker
	t.invoker = &toolInvokerAdapter{invoker: invoker}
}

// toolInvokerAdapter adapts the exported ToolInvoker interface to the internal toolInvoker.
type toolInvokerAdapter struct {
	invoker ToolInvoker
}

func (a *toolInvokerAdapter) invoke(ctx context.Context, toolbox *Toolbox, toolCall ModelToolCall) ModelToolCallResponse {
	return a.invoker.Invoke(ctx, toolbox, toolCall)
}

// NewDirectToolInvoker creates the default production tool invoker.
// Exported for use by the evaluation framework.
func NewDirectToolInvoker() ToolInvoker {
	return &directToolInvoker{}
}

func (t *Toolbox) rejectToolCall(ctx context.Context, toolCall ModelToolCall) ModelToolCallResponse {
	err := fmt.Errorf("tool call %s rejected: %s", toolCall.ToolCallID, toolCall.RejectionReason)
	t.observer.OnToolCall(ctx, toolCall)
	t.observer.OnToolCallError(ctx, toolCall.ToolCallID, err)
	return ModelToolCallResponse{
		ToolCallID: toolCall.ToolCallID,
		Name:       toolCall.FunctionCall.Name,
		Error:      err,
	}
}

func (t *Toolbox) debugConsoleLog(tmpl string, args ...interface{}) {
	if t.debugToolCalls {
		log.Printf(tmpl, args...)
	}
}

func (t *Toolbox) debugAttributes(
	span Span,
	attributes ...Attribute,
) {
	if t.debugToolCalls {
		span.SetAttributes(attributes...)
		log.Printf("Debug attributes: %v", attributes)
	}
}

func (t *Toolbox) executeToolCallStringResponse(
	ctx context.Context,
	toolCall ModelToolCall,
) (string, error) {
	tool, ok := t.tools[toolCall.FunctionCall.Name]
	if !ok {
		return "", fmt.Errorf("tool not found: %s", toolCall.FunctionCall.Name)
	}

	// Start a timer to detect slow tool calls
	start := time.Now()

	// Execute the tool call with context
	toolResponse, err := tool.Invoke(
		ctx,
		toolCall.FunctionCall.Arguments,
		toolCall.ToolCallID,
	)

	// Calculate duration
	duration := time.Since(start)

	if err != nil {
		if ctx.Err() != nil {
			// If the context was canceled or timed out, add more info to the error
			return "", fmt.Errorf("tool call %s timed out after %v: %w (context error: %v)",
				toolCall.FunctionCall.Name, duration, err, ctx.Err())
		}

		// Regular error
		return "", fmt.Errorf("tool call %s failed after %v: %w",
			toolCall.FunctionCall.Name, duration, err)
	}

	return toolResponse, nil
}
