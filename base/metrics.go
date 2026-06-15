package base

import "time"

// ModelCallMetrics holds performance metrics for a single model call.
type ModelCallMetrics struct {
	StartTime      time.Time
	TotalDuration  time.Duration
	ResponseLength int

	IsStreaming               bool
	StreamingTimeToFirstToken time.Duration
	StreamingDuration         time.Duration

	InputTokens  int
	OutputTokens int
	PromptLength int
	MessageCount int

	// Prompt-cache accounting (Anthropic). CacheReadInputTokens is the
	// portion of the prompt served from cache (billed at a fraction of input
	// cost); CacheCreationInputTokens is the portion written to cache. Both
	// are reported separately from InputTokens, which counts only the
	// freshly-processed (uncached) input — so the total prompt size is
	// InputTokens + CacheReadInputTokens + CacheCreationInputTokens. Zero for
	// providers without prompt caching.
	CacheReadInputTokens     int
	CacheCreationInputTokens int
}

// ModelToolCallMetrics holds performance metrics for a single tool call.
type ModelToolCallMetrics struct {
	ToolName      string
	StartTime     time.Time
	TotalDuration time.Duration
}

// ModelToolCallResponse is the result of executing a single tool call.
type ModelToolCallResponse struct {
	// ToolCallID is the ID of the tool call this response is for.
	ToolCallID string `json:"tool_call_id"`
	// Name is the name of the tool that was called.
	Name string `json:"name"`
	// Content is the textual content of the response.
	Content string `json:"content"`
	// Error is the error that occurred while calling the tool.
	Error error `json:"error"`
	// Metrics for this tool call.
	Metrics ModelToolCallMetrics `json:"metrics"`
}
