package hippocampus

import (
	"context"
	"fmt"

	"github.com/yunacaba/hippocampus/base"
)

func NewPromptRequest(
	formattedPrompt FormattedPrompt,
	options ...ModelCallOption,
) ModelCallRequest {
	return NewStreamingPromptRequest(formattedPrompt, nil, options...)
}

func NewStreamingPromptRequest(
	formattedPrompt FormattedPrompt,
	streamingFunc func(ctx context.Context, chunk []byte) error,
	options ...ModelCallOption,
) ModelCallRequest {
	return ModelCallRequest{
		Messages: []ModelMessage{
			NewPromptMessage(formattedPrompt),
		},
		StreamingFunc: streamingFunc,
		Options:       options,
	}
}

func NewStringRequest(
	prompt string,
	options ...ModelCallOption,
) ModelCallRequest {
	return NewStreamingStringRequest(prompt, nil, options...)
}

func NewStreamingStringRequest(
	prompt string,
	streamingFunc func(ctx context.Context, chunk []byte) error,
	options ...ModelCallOption,
) ModelCallRequest {
	return ModelCallRequest{
		Messages: []ModelMessage{
			NewHumanMessage(prompt),
		},
		StreamingFunc: streamingFunc,
		Options:       options,
	}
}

func NewPromptMessage(formattedPrompt FormattedPrompt) ModelMessage {
	prompt := formattedPrompt.Prompt
	if formattedPrompt.ResponseSchema != "" {
		prompt += `

Return JSON with the following schema:
` + formattedPrompt.ResponseSchema
	}
	if formattedPrompt.SampleResponse != "" {
		prompt += `

Sample response:
` + formattedPrompt.SampleResponse
	}
	return NewHumanMessage(prompt)
}

func NewHumanMessage(content string) ModelMessage {
	return base.Message{
		Role:  base.RoleUser,
		Parts: []base.ContentPart{base.TextPart{Text: content}},
	}
}

func NewAgentMessage(content string) ModelMessage {
	return base.Message{
		Role:  base.RoleAssistant,
		Parts: []base.ContentPart{base.TextPart{Text: content}},
	}
}

func NewToolCallMessage(toolCall ModelToolCall) ModelMessage {
	return base.Message{
		Role: base.RoleAssistant,
		Parts: []base.ContentPart{
			base.ToolCallPart{
				ToolCallID: toolCall.ToolCallID,
				Name:       toolCall.FunctionCall.Name,
				Arguments:  toolCall.FunctionCall.Arguments,
			},
		},
	}
}

func NewToolCallResponseMessage(toolCallResponse ModelToolCallResponse) ModelMessage {
	responseContent := toolCallResponse.Content
	if toolCallResponse.Error != nil {
		errorPrefix := "Error: "
		if responseContent != "" {
			errorPrefix = responseContent + "\n\n" + errorPrefix
		}
		responseContent = fmt.Sprintf("%s%s", errorPrefix, toolCallResponse.Error.Error())
	}
	return base.Message{
		Role: base.RoleTool,
		Parts: []base.ContentPart{
			base.ToolResultPart{
				ToolCallID: toolCallResponse.ToolCallID,
				Name:       toolCallResponse.Name,
				Content:    responseContent,
				IsError:    toolCallResponse.Error != nil,
			},
		},
	}
}
