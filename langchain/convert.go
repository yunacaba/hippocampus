package langchain

import (
	"context"

	"github.com/tmc/langchaingo/llms"

	"github.com/yunacaba/hippocampus/base"
)

// roleToLangchain maps a hippocampus role onto a langchaingo chat message type.
func roleToLangchain(role base.Role) llms.ChatMessageType {
	switch role {
	case base.RoleSystem:
		return llms.ChatMessageTypeSystem
	case base.RoleAssistant:
		return llms.ChatMessageTypeAI
	case base.RoleTool:
		return llms.ChatMessageTypeTool
	default:
		return llms.ChatMessageTypeHuman
	}
}

// messagesToLangchain converts owned messages into langchaingo message content.
func messagesToLangchain(messages []base.Message) []llms.MessageContent {
	out := make([]llms.MessageContent, 0, len(messages))
	for _, msg := range messages {
		parts := make([]llms.ContentPart, 0, len(msg.Parts))
		for _, part := range msg.Parts {
			switch p := part.(type) {
			case base.TextPart:
				parts = append(parts, llms.TextContent{Text: p.Text})
			case base.ImagePart:
				parts = append(parts, llms.ImageURLContent{URL: p.URL, Detail: p.Detail})
			case base.BinaryPart:
				parts = append(parts, llms.BinaryContent{MIMEType: p.MIMEType, Data: p.Data})
			case base.ToolCallPart:
				parts = append(parts, llms.ToolCall{
					ID:   p.ToolCallID,
					Type: "function",
					FunctionCall: &llms.FunctionCall{
						Name:      p.Name,
						Arguments: p.Arguments,
					},
				})
			case base.ToolResultPart:
				parts = append(parts, llms.ToolCallResponse{
					ToolCallID: p.ToolCallID,
					Name:       p.Name,
					Content:    p.Content,
				})
			}
		}
		out = append(out, llms.MessageContent{
			Role:  roleToLangchain(msg.Role),
			Parts: parts,
		})
	}
	return out
}

// toolsToLangchain converts owned tool specs into langchaingo tool definitions.
func toolsToLangchain(tools []base.ToolSpec) []llms.Tool {
	out := make([]llms.Tool, 0, len(tools))
	for _, t := range tools {
		out = append(out, llms.Tool{
			Type: "function",
			Function: &llms.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Schema,
			},
		})
	}
	return out
}

// optionsToLangchain resolves owned call options into langchaingo call options.
// The streaming function is wrapped by the caller for TTFT metrics before being
// passed here.
func optionsToLangchain(
	co base.CallOptions,
	streamingFunc func(ctx context.Context, chunk []byte) error,
) []llms.CallOption {
	opts := make([]llms.CallOption, 0)
	if co.Temperature != nil {
		opts = append(opts, llms.WithTemperature(*co.Temperature))
	}
	if co.MaxTokens > 0 {
		opts = append(opts, llms.WithMaxTokens(co.MaxTokens))
	}
	if co.TopP > 0 {
		opts = append(opts, llms.WithTopP(co.TopP))
	}
	if len(co.StopWords) > 0 {
		opts = append(opts, llms.WithStopWords(co.StopWords))
	}
	if co.JSONMode {
		opts = append(opts, llms.WithJSONMode())
	}
	if len(co.Tools) > 0 {
		opts = append(opts, llms.WithTools(toolsToLangchain(co.Tools)))
	}
	switch co.ToolChoice {
	case "":
		// leave unset
	case "auto", "required", "none", "any":
		opts = append(opts, llms.WithToolChoice(co.ToolChoice))
	default: // a specific tool name
		opts = append(opts, llms.WithToolChoice(llms.ToolChoice{
			Type:     "function",
			Function: &llms.FunctionReference{Name: co.ToolChoice},
		}))
	}
	if streamingFunc != nil {
		opts = append(opts, llms.WithStreamingFunc(streamingFunc))
	}
	return opts
}

// responseFromLangchain converts a langchaingo completion into an owned response.
// Token metrics are populated by the caller from GenerationInfo.
func responseFromLangchain(completion *llms.ContentResponse) *base.ModelCallResponse {
	if completion == nil || len(completion.Choices) == 0 {
		return &base.ModelCallResponse{}
	}
	choice := completion.Choices[0]

	toolCalls := make([]base.ModelToolCall, 0, len(choice.ToolCalls))
	for _, tc := range choice.ToolCalls {
		var fc base.FunctionCall
		if tc.FunctionCall != nil {
			fc = base.FunctionCall{Name: tc.FunctionCall.Name, Arguments: tc.FunctionCall.Arguments}
		}
		toolCalls = append(toolCalls, base.ModelToolCall{
			ToolCallID:   tc.ID,
			FunctionCall: fc,
		})
	}

	return &base.ModelCallResponse{
		Content:          choice.Content,
		StopReason:       choice.StopReason,
		GenerationInfo:   choice.GenerationInfo,
		ToolCalls:        toolCalls,
		ReasoningContent: choice.ReasoningContent,
	}
}
