package hippocampus

import (
	"context"

	"github.com/yunacaba/hippocampus/base"
)

// PromptTemplate is the interface for dynamic prompt generation.
type PromptTemplate[TArg any, TResponse any] interface {
	// GeneratePrompt creates a formatted prompt from the given argument
	GeneratePrompt(ctx context.Context, arg TArg) (base.FormattedPrompt, error)

	// GetSampleResponseString returns a sample response string for this prompt type
	GetSampleResponseString() string

	// GetSampleResponseObject returns the typed sample response (for type safety)
	GetSampleResponseObject() TResponse

	// GetResponseSchema returns the JSON schema for responses
	GetResponseSchema() string
}

// FormattedPrompt is re-exported from base for consistency.
type FormattedPrompt = base.FormattedPrompt

// PromptField represents a field in a prompt template (kept for compatibility)
type PromptField struct {
	Name string `json:"name"`
}

// PromptTemplateBase provides common functionality for prompt template implementations
type PromptTemplateBase[TArg any, TResponse any] struct {
	sampleResponse       string
	sampleResponseObject TResponse
	responseSchema       string
}

// GetSampleResponseString returns the sample response.
func (ptb *PromptTemplateBase[TArg, TResponse]) GetSampleResponseString() string {
	return ptb.sampleResponse
}

// GetSampleResponseObject returns the typed sample response.
func (ptb *PromptTemplateBase[TArg, TResponse]) GetSampleResponseObject() TResponse {
	return ptb.sampleResponseObject
}

// GetResponseSchema returns the response schema.
func (ptb *PromptTemplateBase[TArg, TResponse]) GetResponseSchema() string {
	return ptb.responseSchema
}

// NewPromptTemplateBase creates a base with sample response and schema.
func NewPromptTemplateBase[TArg any, TResponse any](
	sampleResponse, responseSchema string,
	sampleResponseObject TResponse,
) *PromptTemplateBase[TArg, TResponse] {
	return &PromptTemplateBase[TArg, TResponse]{
		sampleResponse:       sampleResponse,
		responseSchema:       responseSchema,
		sampleResponseObject: sampleResponseObject,
	}
}
