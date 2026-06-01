package hippocampus

import (
	"github.com/yunacaba/hippocampus/base"
)

// Provider-neutral type aliases re-exported from the base package so callers
// can work with hippocampus.* names.
type (
	Model         = base.Model
	ModelProvider = base.ModelProvider

	ModelCallRequest = base.ModelCallRequest
	ModelCallOption  = base.CallOption
	CallOptions      = base.CallOptions
	ModelMessage     = base.Message
	ModelToolCall    = base.ModelToolCall
	FunctionCall     = base.FunctionCall
	ToolSpec         = base.ToolSpec

	ModelCallResponse     = base.ModelCallResponse
	ModelCallMetrics      = base.ModelCallMetrics
	ModelToolCallMetrics  = base.ModelToolCallMetrics
	ModelToolCallResponse = base.ModelToolCallResponse
)

// Call-option constructors re-exported from base.
var (
	WithTemperature = base.WithTemperature
	WithMaxTokens   = base.WithMaxTokens
	WithTopP        = base.WithTopP
	WithStopWords   = base.WithStopWords
	WithJSONMode    = base.WithJSONMode
	WithTools       = base.WithTools
	WithToolChoice  = base.WithToolChoice
)

type AgentExecutionDetails struct {
	ModelRequests    []ModelCallRequest
	ModelResponses   []ModelCallResponse
	IterationMetrics []AgentIterationMetrics
}

func newAgentExecutionDetails() *AgentExecutionDetails {
	return &AgentExecutionDetails{
		ModelRequests:    []ModelCallRequest{},
		ModelResponses:   []ModelCallResponse{},
		IterationMetrics: []AgentIterationMetrics{},
	}
}

type AgentIterationMetrics struct {
	ToolCallMetrics []ModelToolCallMetrics
}
