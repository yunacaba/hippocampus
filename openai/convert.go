package openai

import (
	"strings"

	oai "github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/shared"

	"github.com/yunacaba/hippocampus/base"
)

// messagesToOpenAI converts owned messages into OpenAI chat completion messages.
func messagesToOpenAI(messages []base.Message) []oai.ChatCompletionMessageParamUnion {
	out := make([]oai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case base.RoleSystem:
			out = append(out, oai.SystemMessage(textOf(msg)))
		case base.RoleTool:
			// A tool-role message carries a single tool result.
			for _, part := range msg.Parts {
				if tr, ok := part.(base.ToolResultPart); ok {
					out = append(out, oai.ToolMessage(tr.Content, tr.ToolCallID))
				}
			}
		case base.RoleAssistant:
			out = append(out, assistantMessage(msg))
		default: // base.RoleUser
			out = append(out, userMessage(msg))
		}
	}
	return out
}

// textOf concatenates the text of all TextParts in a message.
func textOf(msg base.Message) string {
	var b strings.Builder
	for _, part := range msg.Parts {
		if tp, ok := part.(base.TextPart); ok {
			b.WriteString(tp.Text)
		}
	}
	return b.String()
}

// userMessage builds a user message, using a multi-part content array when the
// message contains images, and a plain string otherwise.
func userMessage(msg base.Message) oai.ChatCompletionMessageParamUnion {
	hasRichParts := false
	for _, part := range msg.Parts {
		switch part.(type) {
		case base.ImagePart, base.BinaryPart:
			hasRichParts = true
		}
	}
	if !hasRichParts {
		return oai.UserMessage(textOf(msg))
	}

	parts := make([]oai.ChatCompletionContentPartUnionParam, 0, len(msg.Parts))
	for _, part := range msg.Parts {
		switch p := part.(type) {
		case base.TextPart:
			parts = append(parts, oai.TextContentPart(p.Text))
		case base.ImagePart:
			parts = append(parts, oai.ImageContentPart(oai.ChatCompletionContentPartImageImageURLParam{
				URL:    p.URL,
				Detail: p.Detail,
			}))
		}
	}
	return oai.UserMessage(parts)
}

// assistantMessage builds an assistant message, attaching tool calls when present.
func assistantMessage(msg base.Message) oai.ChatCompletionMessageParamUnion {
	var toolCalls []oai.ChatCompletionMessageToolCallUnionParam
	for _, part := range msg.Parts {
		if tc, ok := part.(base.ToolCallPart); ok {
			toolCalls = append(toolCalls, oai.ChatCompletionMessageToolCallUnionParam{
				OfFunction: &oai.ChatCompletionMessageFunctionToolCallParam{
					ID: tc.ToolCallID,
					Function: oai.ChatCompletionMessageFunctionToolCallFunctionParam{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				},
			})
		}
	}
	if len(toolCalls) == 0 {
		return oai.AssistantMessage(textOf(msg))
	}

	assistant := oai.ChatCompletionAssistantMessageParam{ToolCalls: toolCalls}
	if text := textOf(msg); text != "" {
		assistant.Content = oai.ChatCompletionAssistantMessageParamContentUnion{OfString: oai.String(text)}
	}
	return oai.ChatCompletionMessageParamUnion{OfAssistant: &assistant}
}

// toolsToOpenAI converts owned tool specs into OpenAI function tools.
func toolsToOpenAI(tools []base.ToolSpec) []oai.ChatCompletionToolUnionParam {
	out := make([]oai.ChatCompletionToolUnionParam, 0, len(tools))
	for _, t := range tools {
		out = append(out, oai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
			Name:        t.Name,
			Description: oai.String(t.Description),
			Parameters:  shared.FunctionParameters(t.Schema),
		}))
	}
	return out
}

// applyOptions maps resolved owned call options onto the OpenAI request params.
func applyOptions(params *oai.ChatCompletionNewParams, co base.CallOptions) {
	if co.Temperature != nil {
		params.Temperature = oai.Float(*co.Temperature)
	}
	if co.MaxTokens > 0 {
		params.MaxCompletionTokens = oai.Int(int64(co.MaxTokens))
	}
	if co.TopP > 0 {
		params.TopP = oai.Float(co.TopP)
	}
	if len(co.StopWords) > 0 {
		params.Stop = oai.ChatCompletionNewParamsStopUnion{OfStringArray: co.StopWords}
	}
	// A response schema is stronger than plain JSON mode, so it takes precedence.
	switch {
	case co.ResponseSchema != nil:
		js := shared.ResponseFormatJSONSchemaJSONSchemaParam{
			Name:   co.ResponseSchema.Name,
			Schema: co.ResponseSchema.Schema,
		}
		if co.ResponseSchema.Strict {
			js.Strict = oai.Bool(true)
		}
		params.ResponseFormat = oai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{JSONSchema: js},
		}
	case co.JSONMode:
		params.ResponseFormat = oai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: &shared.ResponseFormatJSONObjectParam{},
		}
	}
	if len(co.Tools) > 0 {
		params.Tools = toolsToOpenAI(co.Tools)
	}
	switch co.ToolChoice {
	case "":
		// leave unset (OpenAI defaults to "auto" when tools are present)
	case "auto", "required", "none":
		params.ToolChoice = oai.ChatCompletionToolChoiceOptionUnionParam{OfAuto: oai.String(co.ToolChoice)}
	default: // a specific tool name
		params.ToolChoice = oai.ChatCompletionToolChoiceOptionUnionParam{
			OfFunctionToolChoice: &oai.ChatCompletionNamedToolChoiceParam{
				Function: oai.ChatCompletionNamedToolChoiceFunctionParam{Name: co.ToolChoice},
			},
		}
	}
}

// responseFromOpenAI converts an OpenAI chat completion into an owned response.
func responseFromOpenAI(completion *oai.ChatCompletion) *base.ModelCallResponse {
	if completion == nil || len(completion.Choices) == 0 {
		return &base.ModelCallResponse{}
	}
	choice := completion.Choices[0]
	msg := choice.Message

	toolCalls := make([]base.ModelToolCall, 0, len(msg.ToolCalls))
	for _, tc := range msg.ToolCalls {
		if tc.Type != "" && tc.Type != "function" {
			continue
		}
		toolCalls = append(toolCalls, base.ModelToolCall{
			ToolCallID: tc.ID,
			FunctionCall: base.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}

	return &base.ModelCallResponse{
		Content:    msg.Content,
		StopReason: choice.FinishReason,
		ToolCalls:  toolCalls,
		GenerationInfo: map[string]any{
			"InputTokens":  int(completion.Usage.PromptTokens),
			"OutputTokens": int(completion.Usage.CompletionTokens),
		},
	}
}
