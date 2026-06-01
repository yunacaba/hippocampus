package anthropic

import (
	"encoding/json"
	"strings"

	sdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/yunacaba/hippocampus/base"
)

// defaultMaxTokens is used when the caller does not specify a token budget.
// Anthropic requires max_tokens on every request.
const defaultMaxTokens = 4096

// splitMessages converts owned messages into Anthropic messages, hoisting all
// system-role content into the returned top-level system blocks (the Messages
// API has no system role).
func splitMessages(messages []base.Message) (system []sdk.TextBlockParam, msgs []sdk.MessageParam) {
	for _, msg := range messages {
		if msg.Role == base.RoleSystem {
			if text := textOf(msg); text != "" {
				system = append(system, sdk.TextBlockParam{Text: text})
			}
			continue
		}
		msgs = append(msgs, messageToAnthropic(msg))
	}
	return system, msgs
}

func textOf(msg base.Message) string {
	var b strings.Builder
	for _, part := range msg.Parts {
		if tp, ok := part.(base.TextPart); ok {
			b.WriteString(tp.Text)
		}
	}
	return b.String()
}

// messageToAnthropic converts a single non-system message.
func messageToAnthropic(msg base.Message) sdk.MessageParam {
	blocks := make([]sdk.ContentBlockParamUnion, 0, len(msg.Parts))
	for _, part := range msg.Parts {
		switch p := part.(type) {
		case base.TextPart:
			blocks = append(blocks, sdk.NewTextBlock(p.Text))
		case base.ImagePart:
			blocks = append(blocks, sdk.NewImageBlock(sdk.URLImageSourceParam{URL: p.URL}))
		case base.ToolCallPart:
			// Arguments are JSON-encoded; pass them through as raw JSON input.
			var input any = json.RawMessage(p.Arguments)
			if p.Arguments == "" {
				input = json.RawMessage("{}")
			}
			blocks = append(blocks, sdk.NewToolUseBlock(p.ToolCallID, input, p.Name))
		case base.ToolResultPart:
			blocks = append(blocks, sdk.NewToolResultBlock(p.ToolCallID, p.Content, p.IsError))
		}
	}

	if msg.Role == base.RoleAssistant {
		return sdk.NewAssistantMessage(blocks...)
	}
	// RoleUser and RoleTool both map to user turns; tool results are user-role
	// content blocks in the Anthropic Messages API.
	return sdk.NewUserMessage(blocks...)
}

// toolsToAnthropic converts owned tool specs into Anthropic tools.
func toolsToAnthropic(tools []base.ToolSpec) []sdk.ToolUnionParam {
	out := make([]sdk.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		schema := sdk.ToolInputSchemaParam{}
		if props, ok := t.Schema["properties"]; ok {
			schema.Properties = props
		}
		if req, ok := t.Schema["required"].([]any); ok {
			required := make([]string, 0, len(req))
			for _, r := range req {
				if s, ok := r.(string); ok {
					required = append(required, s)
				}
			}
			schema.Required = required
		}
		toolParam := &sdk.ToolParam{
			Name:        t.Name,
			InputSchema: schema,
		}
		if t.Description != "" {
			toolParam.Description = sdk.String(t.Description)
		}
		out = append(out, sdk.ToolUnionParam{OfTool: toolParam})
	}
	return out
}

// applyOptions maps resolved owned call options onto the Anthropic request.
func applyOptions(params *sdk.MessageNewParams, co base.CallOptions) {
	if co.MaxTokens > 0 {
		params.MaxTokens = int64(co.MaxTokens)
	} else {
		params.MaxTokens = defaultMaxTokens
	}
	if co.Temperature != nil {
		params.Temperature = sdk.Float(*co.Temperature)
	}
	if co.TopP > 0 {
		params.TopP = sdk.Float(co.TopP)
	}
	if len(co.StopWords) > 0 {
		params.StopSequences = co.StopWords
	}
	if len(co.Tools) > 0 {
		params.Tools = toolsToAnthropic(co.Tools)
	}
	// Anthropic has no dedicated JSON-mode flag; steer it with a system
	// instruction so JSONMode is honored rather than silently dropped.
	if co.JSONMode {
		params.System = append(params.System, sdk.TextBlockParam{
			Text: "Respond with only a single valid JSON object and no other text.",
		})
	}
}

// responseFromAnthropic converts an Anthropic message into an owned response.
func responseFromAnthropic(msg *sdk.Message) *base.ModelCallResponse {
	if msg == nil {
		return &base.ModelCallResponse{}
	}

	var content strings.Builder
	var toolCalls []base.ModelToolCall
	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			content.WriteString(block.Text)
		case "tool_use":
			toolCalls = append(toolCalls, base.ModelToolCall{
				ToolCallID: block.ID,
				FunctionCall: base.FunctionCall{
					Name:      block.Name,
					Arguments: string(block.Input),
				},
			})
		}
	}

	return &base.ModelCallResponse{
		Content:    content.String(),
		StopReason: string(msg.StopReason),
		ToolCalls:  toolCalls,
		GenerationInfo: map[string]any{
			"InputTokens":  int(msg.Usage.InputTokens),
			"OutputTokens": int(msg.Usage.OutputTokens),
		},
	}
}
