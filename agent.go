package hippocampus

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yunacaba/hippocampus/jsonx"
)

// ToolCallingPolicy defines when an agent is allowed to make tool calls across iterations.
type ToolCallingPolicy int

const (
	// ToolCallingAnyIteration allows tool calls in any iteration (default behavior)
	ToolCallingAnyIteration ToolCallingPolicy = iota

	// ToolCallingFirstIterationOnly allows tool calls only in the first iteration
	ToolCallingFirstIterationOnly

	// ToolCallingFirstIterationWithRetries allows tool calls in first iteration, then only on errors
	ToolCallingFirstIterationWithRetries
)

// Agent controls the flow of a single request with tool calls,
// and it continues to call the model until there are no tool
// calls in the response (or maxIterations is reached).
//
// For more complex flows, create a higher-level orchestrator
// that uses multiple agents.
//
// Agents:
// - are 1:1 with a prompt template
// - are 1:1 with an LLM model
// - can contain multiple tools
// - are idempotent
//
// Concurrency: an Agent holds only immutable configuration after Build; each
// Execute keeps its mutable state (pending messages, execution details) local.
// It is therefore safe to call Execute concurrently from multiple goroutines,
// provided the injected Model, ToolDelegate, AgentObserver, and tool functions
// are themselves concurrency-safe. The framework also invokes observer and tool
// callbacks concurrently when a single response contains multiple tool calls.
type Agent[TI any, TO any] struct {
	name              string
	model             Model
	observer          AgentObserver[TO]
	toolbox           *Toolbox
	promptTemplate    PromptTemplate[TI, TO]
	tracer            Tracer
	toolCallingPolicy ToolCallingPolicy

	debugToolCalls bool
	maxIterations  int
}

// newAgent creates an agent using the PromptTemplate interface.
func newAgent[TI any, TO any](
	name string,
	model Model,
	observer AgentObserver[TO],
	delegate ToolDelegate,
	promptTemplate PromptTemplate[TI, TO],
	tools []AnyTool,
	tracer Tracer,
) (*Agent[TI, TO], error) {
	if tracer == nil {
		tracer = NoopTracer{}
	}
	toolbox, err := newToolbox(name, tools, observer, delegate, tracer, false)
	if err != nil {
		return nil, err
	}
	if observer == nil {
		observer = newNoOpAgentObserver[TO]()
	}

	return &Agent[TI, TO]{
		name:           name,
		model:          model,
		observer:       observer,
		toolbox:        toolbox,
		promptTemplate: promptTemplate,
		tracer:         tracer,
		debugToolCalls: false,
		maxIterations:  5,
	}, nil
}

// newAgentWithTextTemplate creates an agent using a text template string.
func newAgentWithTextTemplate[TI any, TO any](
	name string,
	model Model,
	observer AgentObserver[TO],
	delegate ToolDelegate,
	templateText string,
	sampleArg TI,
	sampleResponse TO,
	tools []AnyTool,
	tracer Tracer,
) (*Agent[TI, TO], error) {
	if tracer == nil {
		tracer = NoopTracer{}
	}
	promptTemplate, err := NewTextPromptTemplate(templateText, sampleArg, sampleResponse)
	if err != nil {
		return nil, err
	}
	toolbox, err := newToolbox(name, tools, observer, delegate, tracer, false)
	if err != nil {
		return nil, err
	}
	if observer == nil {
		observer = newNoOpAgentObserver[TO]()
	}
	return &Agent[TI, TO]{
		name:           name,
		model:          model,
		observer:       observer,
		toolbox:        toolbox,
		promptTemplate: promptTemplate,
		tracer:         tracer,
		debugToolCalls: false,
		maxIterations:  5,
	}, nil
}

// buildIterationMessage creates the iteration guidance message based on tool calling policy.
func (a *Agent[TI, TO]) buildIterationMessage(iteration int, maxIterations int) string {
	iterationMessage := fmt.Sprintf("Iteration %d of %d.", iteration+1, maxIterations)

	// Apply tool calling policy guidance after first iteration
	if iteration > 0 {
		switch a.toolCallingPolicy {
		case ToolCallingFirstIterationWithRetries:
			iterationMessage += " You already made your initial tool calls. Generate your final response based on the tool results. Do not make additional tool calls unless a tool returned an error that requires retry."
		case ToolCallingFirstIterationOnly:
			iterationMessage += " You already made your tool calls. Generate your final response. Do not make any additional tool calls."
		case ToolCallingAnyIteration:
			// No additional guidance - allow tools in any iteration
		}
	}

	if (iteration + 1) == maxIterations {
		iterationMessage += " Final iteration; any tool calls will fail."
	}

	return iterationMessage
}

func (a *Agent[TI, TO]) Execute(
	ctx context.Context,
	arg TI,
	streamingFunc func(ctx context.Context, chunk []byte) error,
	options ...ModelCallOption,
) (TO, error) {
	ptr, _, err := a.ExecuteWithDetails(ctx, arg, streamingFunc, options...)
	return ptr, err
}

func (a *Agent[TI, TO]) ExecuteWithDetails(
	ctx context.Context,
	arg TI,
	streamingFunc func(ctx context.Context, chunk []byte) error,
	options ...ModelCallOption,
) (TO, *AgentExecutionDetails, error) {
	// Create the span with the original context
	ctx, span := a.tracer.StartSpan(ctx, fmt.Sprintf("%s.Agent.Execute", a.name))
	defer span.End()

	details := newAgentExecutionDetails()

	// Use the provided context for agent execution to preserve cancellation chain
	// This ensures proper request cancellation propagates through agent operations
	agentCtx := ctx

	// Wrap streamingFunc to use the original context for callbacks
	// This ensures UI updates work even with our detached execution context
	wrappedStreamingFunc := streamingFunc
	if streamingFunc != nil {
		wrappedStreamingFunc = func(_ context.Context, chunk []byte) error {
			// Always call the original streaming function with the original context
			return streamingFunc(ctx, chunk)
		}
	}

	var zero TO

	formattedPrompt, err := a.promptTemplate.GeneratePrompt(ctx, arg)
	if err != nil {
		return zero, nil, err
	}

	additionalOptions := a.optionsForPrompt(&formattedPrompt)
	options = append(options, additionalOptions...)

	pendingMessages := []ModelMessage{
		NewPromptMessage(formattedPrompt),
	}

	maxIterations := a.maxIterations
	for i := 0; i < maxIterations; i++ {
		// Check if the original context was canceled to allow for graceful cancellation
		select {
		case <-ctx.Done():
			return zero, details, fmt.Errorf("operation was canceled: %w", ctx.Err())
		default:
			// Continue execution
		}

		messagesForRequest := pendingMessages
		if len(a.toolbox.Tools()) > 0 {
			iterationMessage := a.buildIterationMessage(i, maxIterations)
			a.debugConsoleLog("Iteration: %s", iterationMessage)
			messagesForRequest = append(
				messagesForRequest,
				NewHumanMessage(iterationMessage),
			)
		}

		// Use a timeout context just for this LLM call
		modelCtx, cancelModel := context.WithTimeout(agentCtx, 60*time.Second)
		a.observer.OnLLMCall(modelCtx)
		request := ModelCallRequest{
			Messages:      messagesForRequest,
			Options:       options,
			StreamingFunc: wrappedStreamingFunc,
		}
		details.ModelRequests = append(details.ModelRequests, request)
		response, err := a.model.Generate(modelCtx, request)

		// Always cancel the timeout context when done with this call
		cancelModel()

		if err != nil {
			a.observer.OnLLMError(modelCtx, err)
			return zero, details, err
		}

		details.ModelResponses = append(details.ModelResponses, *response)
		a.debugConsoleLog("Response: %v", response.Content)
		responseContent, err := a.parseResponse(response.Content)
		if err == nil {
			a.observer.OnLLMResponse(modelCtx, responseContent)
		}

		// If no tool calls, return immediately.
		if len(response.ToolCalls) == 0 {
			// If there are tool calls, the LLM may not return a response,
			// so ignore parse errors unless we're about to return.
			if err != nil {
				a.observer.OnLLMError(modelCtx, err)
				return zero, details, err
			}
			return responseContent, details, nil
		}

		// Making tool calls, add content to history.
		if len(response.Content) > 0 {
			pendingMessages = append(pendingMessages, NewAgentMessage(response.Content))
		}

		// Execute tool calls with separate timeout contexts.
		toolCallResult := a.toolbox.executeLLMToolCalls(agentCtx, response.ToolCalls)
		a.debugConsoleLog(
			"Tool calls: %v (%d count)",
			toolCallResult.pendingMessages,
			len(toolCallResult.toolResponses),
		)

		// If no sync tool calls, return immediately.
		// Nothing to notify LLM about.
		if len(toolCallResult.toolResponses) == 0 {
			a.debugConsoleLog("No sync tool calls, returning response content")
			return responseContent, details, nil
		}

		// Add tool call messages to history.
		pendingMessages = append(pendingMessages, toolCallResult.pendingMessages...)
		details.IterationMetrics = append(
			details.IterationMetrics,
			AgentIterationMetrics{
				ToolCallMetrics: toolCallResult.toolCallMetrics,
			},
		)
	}
	return zero, details, fmt.Errorf("max iterations reached")
}

func (a *Agent[TI, TO]) optionsForPrompt(formattedPrompt *FormattedPrompt) []ModelCallOption {
	options := make([]ModelCallOption, 0)

	// Set temperature based on model capabilities
	llmType := a.model.LLMType()
	if concrete, ok := llmType.(LLMType); ok && concrete.SupportsCustomTemperature() {
		// Use temperature=0 for consistent results
		options = append(options, WithTemperature(0.0))
	} else {
		// Use default temperature=1.0 for models that don't support custom values
		options = append(options, WithTemperature(1.0))
	}

	sampleResponse := formattedPrompt.SampleResponse
	if sampleResponse != "" &&
		strings.Contains(sampleResponse, "{") &&
		strings.Contains(sampleResponse, "}") {
		options = append(options, WithJSONMode())
	}
	toolCount := len(a.toolbox.Tools())
	if toolCount > 0 {
		tools := make([]ToolSpec, 0, toolCount)
		for _, tool := range a.toolbox.Tools() {
			tools = append(tools, tool.toToolSpec())
		}
		options = append(options, WithTools(tools))
	}
	return options
}

func (a *Agent[TI, TO]) parseResponse(responseString string) (TO, error) {
	var zero TO
	if responseString == "" {
		return zero, fmt.Errorf("empty response")
	}
	responseString = strings.TrimPrefix(responseString, "```json")
	responseString = strings.TrimSuffix(responseString, "```")

	response, err := jsonx.Deserialize[TO](responseString, a.promptTemplate.GetSampleResponseObject())
	if err != nil {
		return zero, fmt.Errorf("failed to parse response JSON: %w (raw response: %q)", err, responseString)
	}

	return response, nil
}

func (a *Agent[PTA, PTR]) PromptTemplate() PromptTemplate[PTA, PTR] {
	return a.promptTemplate
}

func (a *Agent[TI, TO]) debugConsoleLog(tmpl string, args ...interface{}) {
	if a.debugToolCalls {
		log.Printf(tmpl, args...)
	}
}

// SetToolInvoker replaces the toolbox's invoker (for evaluation framework).
func (a *Agent[TI, TO]) SetToolInvoker(invoker ToolInvoker) {
	a.toolbox.SetToolInvoker(invoker)
}
