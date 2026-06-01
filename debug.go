package hippocampus

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yunacaba/hippocampus/base"
)

const debugDir = "/tmp/hippocampus-agent-debug"

var (
	debugEnabled     bool
	debugEnabledOnce sync.Once
)

// DebugMetadata contains correlation context for debug output.
type DebugMetadata struct {
	Timestamp string `json:"timestamp"`
	UserID    string `json:"user_id,omitempty"`
}

// MessageDebugInfo contains message information for debug output.
type MessageDebugInfo struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToolDebugInfo contains tool definition information.
type ToolDebugInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// PromptDebugData contains all prompt information for debugging.
type PromptDebugData struct {
	Metadata   DebugMetadata      `json:"metadata"`
	AgentName  string             `json:"agent_name"`
	ModelType  string             `json:"model_type"`
	Messages   []MessageDebugInfo `json:"messages"`
	Tools      []ToolDebugInfo    `json:"tools,omitempty"`
	TokenCount int                `json:"token_count"`
}

// ToolCallDebugInfo contains information about tool calls in the response.
type ToolCallDebugInfo struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ResponseDebugData contains all response information for debugging.
type ResponseDebugData struct {
	Metadata      DebugMetadata       `json:"metadata"`
	AgentName     string              `json:"agent_name"`
	ModelType     string              `json:"model_type"`
	Content       string              `json:"content"`
	InputTokens   int                 `json:"input_tokens,omitempty"`
	OutputTokens  int                 `json:"output_tokens,omitempty"`
	DurationMs    int64               `json:"duration_ms"`
	ToolCalls     []ToolCallDebugInfo `json:"tool_calls,omitempty"`
	ToolCallCount int                 `json:"tool_call_count"`
}

// IsDebugEnabled checks if agent debugging is enabled via the
// AGENT_DEBUG_PROMPTS environment variable. Uses sync.Once to cache the result.
func IsDebugEnabled() bool {
	debugEnabledOnce.Do(func() {
		debugEnabled = strings.ToLower(os.Getenv("AGENT_DEBUG_PROMPTS")) == "true"
		if debugEnabled {
			// Ensure directory exists
			if err := os.MkdirAll(debugDir, 0o755); err != nil {
				fmt.Fprintf(os.Stderr, "agent debug: failed to create debug directory: %v\n", err)
				debugEnabled = false
			}
		}
	})
	return debugEnabled
}

// extractMetadata extracts correlation context from the context.
func extractMetadata(ctx context.Context) DebugMetadata {
	metadata := DebugMetadata{
		Timestamp: time.Now().Format(time.RFC3339Nano),
	}
	if userID, ok := UserIDFromContext(ctx); ok {
		metadata.UserID = userID
	}
	return metadata
}

// extractMessagesFromRequest extracts message information from the request.
func extractMessagesFromRequest(messages []ModelMessage) []MessageDebugInfo {
	result := make([]MessageDebugInfo, 0, len(messages))
	for _, msg := range messages {
		content := extractContentFromMessage(msg)
		result = append(result, MessageDebugInfo{
			Role:    string(msg.Role),
			Content: content,
		})
	}
	return result
}

// extractContentFromMessage extracts a human-readable summary of a message's parts.
func extractContentFromMessage(msg ModelMessage) string {
	var parts []string
	for _, part := range msg.Parts {
		switch p := part.(type) {
		case base.TextPart:
			parts = append(parts, p.Text)
		case base.ImagePart:
			parts = append(parts, fmt.Sprintf("[Image: %s]", p.URL))
		case base.BinaryPart:
			parts = append(parts, fmt.Sprintf("[Binary: %s (%d bytes)]", p.MIMEType, len(p.Data)))
		case base.ToolCallPart:
			parts = append(parts, fmt.Sprintf("[ToolCall: %s(%s)]", p.Name, p.Arguments))
		case base.ToolResultPart:
			parts = append(parts, fmt.Sprintf("[ToolResult: %s]", p.Content))
		default:
			parts = append(parts, fmt.Sprintf("[Unknown part type: %T]", part))
		}
	}
	return strings.Join(parts, "\n")
}

// extractToolsFromOptions extracts tool definitions from call options.
func extractToolsFromOptions(options []ModelCallOption) []ToolDebugInfo {
	var tools []ToolDebugInfo

	co := base.ResolveCallOptions(options)
	for _, tool := range co.Tools {
		tools = append(tools, ToolDebugInfo{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Schema,
		})
	}

	return tools
}

// LogPromptDebug logs the prompt before sending to the LLM.
func LogPromptDebug(ctx context.Context, agentName, modelType string, request ModelCallRequest) {
	if !IsDebugEnabled() {
		return
	}

	metadata := extractMetadata(ctx)
	messages := extractMessagesFromRequest(request.Messages)
	tools := extractToolsFromOptions(request.Options)

	data := PromptDebugData{
		Metadata:   metadata,
		AgentName:  agentName,
		ModelType:  modelType,
		Messages:   messages,
		Tools:      tools,
		TokenCount: request.Length(),
	}

	// Build text format
	var textBuilder strings.Builder
	textBuilder.WriteString("=== Agent Prompt Debug ===\n")
	textBuilder.WriteString(fmt.Sprintf("Timestamp: %s\n", metadata.Timestamp))
	textBuilder.WriteString(fmt.Sprintf("Agent: %s\n", agentName))
	textBuilder.WriteString(fmt.Sprintf("Model: %s\n", modelType))
	if metadata.UserID != "" {
		textBuilder.WriteString(fmt.Sprintf("User ID: %s\n", metadata.UserID))
	}
	textBuilder.WriteString(fmt.Sprintf("Message Count: %d\n", len(messages)))
	textBuilder.WriteString(fmt.Sprintf("Estimated Tokens: %d\n", data.TokenCount))
	if len(tools) > 0 {
		textBuilder.WriteString(fmt.Sprintf("Tool Count: %d\n", len(tools)))
	}
	textBuilder.WriteString("\n")

	// Add tool definitions
	if len(tools) > 0 {
		textBuilder.WriteString("=== Tool Definitions ===\n\n")
		for i, tool := range tools {
			textBuilder.WriteString(fmt.Sprintf("[%d] %s\n", i+1, tool.Name))
			if tool.Description != "" {
				textBuilder.WriteString(fmt.Sprintf("    Description: %s\n", tool.Description))
			}
			if tool.Parameters != nil {
				paramsJSON, _ := json.MarshalIndent(tool.Parameters, "    ", "  ")
				textBuilder.WriteString(fmt.Sprintf("    Parameters: %s\n", string(paramsJSON)))
			}
			textBuilder.WriteString("\n")
		}
	}

	// Add messages
	textBuilder.WriteString("=== Messages ===\n\n")
	for i, msg := range messages {
		textBuilder.WriteString(fmt.Sprintf("[%d] %s:\n%s\n\n", i+1,
			strings.ToUpper(msg.Role), msg.Content))
	}

	writeDebugFiles("prompt", data, textBuilder.String())
}

// LogResponseDebug logs the response after receiving from the LLM.
func LogResponseDebug(ctx context.Context, agentName, modelType string, response *ModelCallResponse, metrics *ModelCallMetrics) {
	if !IsDebugEnabled() {
		return
	}

	metadata := extractMetadata(ctx)

	var toolCalls []ToolCallDebugInfo
	for _, tc := range response.ToolCalls {
		toolCalls = append(toolCalls, ToolCallDebugInfo{
			Name:      tc.FunctionCall.Name,
			Arguments: tc.FunctionCall.Arguments,
		})
	}

	var inputTokens, outputTokens int
	var durationMs int64
	if metrics != nil {
		inputTokens = metrics.InputTokens
		outputTokens = metrics.OutputTokens
		durationMs = metrics.TotalDuration.Milliseconds()
	}

	data := ResponseDebugData{
		Metadata:      metadata,
		AgentName:     agentName,
		ModelType:     modelType,
		Content:       response.Content,
		InputTokens:   inputTokens,
		OutputTokens:  outputTokens,
		DurationMs:    durationMs,
		ToolCalls:     toolCalls,
		ToolCallCount: len(toolCalls),
	}

	// Build text format
	var textBuilder strings.Builder
	textBuilder.WriteString("=== Agent Response Debug ===\n")
	textBuilder.WriteString(fmt.Sprintf("Timestamp: %s\n", metadata.Timestamp))
	textBuilder.WriteString(fmt.Sprintf("Agent: %s\n", agentName))
	textBuilder.WriteString(fmt.Sprintf("Model: %s\n", modelType))
	textBuilder.WriteString(fmt.Sprintf("Duration: %dms\n", data.DurationMs))
	if inputTokens > 0 {
		textBuilder.WriteString(fmt.Sprintf("Input Tokens: %d\n", inputTokens))
	}
	if outputTokens > 0 {
		textBuilder.WriteString(fmt.Sprintf("Output Tokens: %d\n", outputTokens))
	}
	textBuilder.WriteString(fmt.Sprintf("Tool Calls: %d\n", data.ToolCallCount))
	textBuilder.WriteString("\n")

	textBuilder.WriteString("=== Response Content ===\n")
	textBuilder.WriteString(response.Content)
	textBuilder.WriteString("\n")

	if len(toolCalls) > 0 {
		textBuilder.WriteString("\n=== Tool Calls ===\n")
		for i, tc := range toolCalls {
			textBuilder.WriteString(fmt.Sprintf("[%d] %s(%s)\n", i+1, tc.Name, tc.Arguments))
		}
	}

	writeDebugFiles("response", data, textBuilder.String())
}

// writeDebugFiles writes both .txt and .json debug files.
func writeDebugFiles(prefix string, data interface{}, textContent string) {
	// Ensure debug directory exists (may have been removed by tests or external process)
	if err := os.MkdirAll(debugDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "agent debug: failed to create debug directory: %v\n", err)
		return
	}

	timestamp := time.Now().Format("20060102_150405.000000")
	baseName := fmt.Sprintf("%s_%s", prefix, timestamp)

	// Write .txt file (0o600 = owner read/write only, for security)
	txtPath := filepath.Join(debugDir, baseName+".txt")
	if err := os.WriteFile(txtPath, []byte(textContent), 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "agent debug: failed to write %s: %v\n", txtPath, err)
	}

	// Write .json file (0o600 = owner read/write only, for security)
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "agent debug: failed to marshal JSON: %v\n", err)
		return
	}

	jsonPath := filepath.Join(debugDir, baseName+".json")
	if err := os.WriteFile(jsonPath, jsonData, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "agent debug: failed to write %s: %v\n", jsonPath, err)
	}
}
