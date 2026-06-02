package base

import "context"

// ToolSpec describes a tool made available to the model on a call. The Schema
// is a JSON Schema object describing the tool's parameters.
type ToolSpec struct {
	Name        string
	Description string
	Schema      map[string]any
}

// ResponseSchema requests that the model's output conform to a JSON Schema.
// Adapters map it to their native structured-output mechanism (OpenAI
// response_format json_schema; Anthropic a forced output tool). Adapters that
// cannot enforce it ignore it and rely on prompt guidance.
type ResponseSchema struct {
	// Name is the provider-visible schema name (e.g. "response"). OpenAI
	// requires it to match [a-zA-Z0-9_-] and be at most 64 characters.
	Name string
	// Schema is the JSON Schema object describing the desired output.
	Schema map[string]any
	// Strict requests strict schema adherence where supported (OpenAI). The
	// schema must satisfy the provider's strict-mode subset.
	Strict bool
}

// CallOptions is the resolved set of options for a single model call. Each
// Model adapter inspects it and maps the relevant fields onto its provider
// SDK's request. Provider-neutral; replaces langchaingo's llms.CallOptions.
type CallOptions struct {
	// Temperature is the sampling temperature. Nil means "leave unset / use
	// the provider default" so adapters can distinguish unset from 0.
	Temperature *float64
	MaxTokens   int
	TopP        float64
	StopWords   []string
	Tools       []ToolSpec
	// ToolChoice is "auto", "required", "none", or a specific tool name.
	ToolChoice string
	// JSONMode requests that the model emit a JSON object.
	JSONMode bool
	// ResponseSchema, when set, requests schema-conformant output.
	ResponseSchema *ResponseSchema
}

// CallOption is a functional option applied to a model call. Replaces
// llms.CallOption.
type CallOption func(*CallOptions)

// ResolveCallOptions applies the given options onto a zero CallOptions and
// returns the result. Adapters call this once at the top of Generate.
func ResolveCallOptions(opts []CallOption) CallOptions {
	var co CallOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&co)
		}
	}
	return co
}

// WithTemperature sets the sampling temperature.
func WithTemperature(t float64) CallOption {
	return func(o *CallOptions) { o.Temperature = &t }
}

// WithMaxTokens sets the maximum number of tokens to generate.
func WithMaxTokens(n int) CallOption {
	return func(o *CallOptions) { o.MaxTokens = n }
}

// WithTopP sets nucleus sampling.
func WithTopP(p float64) CallOption {
	return func(o *CallOptions) { o.TopP = p }
}

// WithStopWords sets stop sequences.
func WithStopWords(words []string) CallOption {
	return func(o *CallOptions) { o.StopWords = words }
}

// WithJSONMode requests that the model emit a JSON object.
func WithJSONMode() CallOption {
	return func(o *CallOptions) { o.JSONMode = true }
}

// WithTools sets the tools available to the model on this call.
func WithTools(tools []ToolSpec) CallOption {
	return func(o *CallOptions) { o.Tools = tools }
}

// WithToolChoice sets the tool-choice policy: "auto", "required", "none", or a
// specific tool name.
func WithToolChoice(choice string) CallOption {
	return func(o *CallOptions) { o.ToolChoice = choice }
}

// WithResponseSchema requests that the model's output conform to the given JSON
// Schema, named for the provider. Non-strict; see WithStrictResponseSchema.
func WithResponseSchema(name string, schema map[string]any) CallOption {
	return func(o *CallOptions) {
		o.ResponseSchema = &ResponseSchema{Name: name, Schema: schema}
	}
}

// WithStrictResponseSchema is like WithResponseSchema but requests strict
// adherence where the provider supports it (currently OpenAI). The schema must
// satisfy the provider's strict-mode subset.
func WithStrictResponseSchema(name string, schema map[string]any) CallOption {
	return func(o *CallOptions) {
		o.ResponseSchema = &ResponseSchema{Name: name, Schema: schema, Strict: true}
	}
}

// ModelCallRequest is the input to Model.Generate.
type ModelCallRequest struct {
	Messages      []Message
	Options       []CallOption
	StreamingFunc func(ctx context.Context, chunk []byte) error
}

// Length returns the approximate prompt size, in bytes of content, across all
// messages and parts. Used for metrics.
func (r *ModelCallRequest) Length() int {
	totalLength := 0
	for _, message := range r.Messages {
		for _, part := range message.Parts {
			switch p := part.(type) {
			case TextPart:
				totalLength += len(p.Text)
			case ImagePart:
				totalLength += len(p.URL)
			case BinaryPart:
				totalLength += len(p.Data)
			case ToolCallPart:
				totalLength += len(p.Arguments)
			case ToolResultPart:
				totalLength += len(p.Content)
			}
		}
	}
	return totalLength
}

// ModelCallResponse is the output of Model.Generate.
type ModelCallResponse struct {
	// Content is the textual content of a response.
	Content string

	// StopReason is the reason the model stopped generating output.
	StopReason string

	// GenerationInfo is arbitrary information the model adds to the response.
	GenerationInfo map[string]any

	// ToolCalls is a list of tool calls the model asks to invoke.
	ToolCalls []ModelToolCall

	// ReasoningContent is the reasoning contents of the assistant message
	// before the final answer (used by reasoning models).
	ReasoningContent string

	// Metrics holds performance metrics for the call.
	Metrics *ModelCallMetrics
}
