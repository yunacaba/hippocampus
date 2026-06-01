package hippocampus

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yunacaba/hippocampus/base"
)

func TestDebugLogging(t *testing.T) {
	os.Setenv("AGENT_DEBUG_PROMPTS", "true")
	defer os.Unsetenv("AGENT_DEBUG_PROMPTS")

	// Reset the once flag for testing
	debugEnabledOnce = sync.Once{}
	os.RemoveAll(debugDir)
	defer os.RemoveAll(debugDir)

	if !IsDebugEnabled() {
		t.Fatal("Debug mode should be enabled")
	}

	ctx := WithUserID(context.Background(), "test_user_123")

	request := ModelCallRequest{
		Messages: []ModelMessage{
			{
				Role:  base.RoleSystem,
				Parts: []base.ContentPart{base.TextPart{Text: "You are a helpful assistant."}},
			},
			{
				Role:  base.RoleUser,
				Parts: []base.ContentPart{base.TextPart{Text: "Hello, can you help me?"}},
			},
		},
		Options: []ModelCallOption{
			WithTools([]ToolSpec{
				{
					Name:        "test_tool",
					Description: "A test tool for debugging",
					Schema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"query": map[string]any{
								"type":        "string",
								"description": "The search query",
							},
						},
					},
				},
			}),
		},
	}

	LogPromptDebug(ctx, "TestAgent", "test-model", request)

	response := &ModelCallResponse{
		Content: "Hello! I'd be happy to help you.",
		ToolCalls: []base.ModelToolCall{
			{
				ToolCallID:   "1",
				FunctionCall: base.FunctionCall{Name: "test_tool", Arguments: `{"query": "test"}`},
			},
		},
	}
	metrics := &ModelCallMetrics{
		TotalDuration: 1234 * time.Millisecond,
		InputTokens:   100,
		OutputTokens:  50,
	}

	LogResponseDebug(ctx, "TestAgent", "test-model", response, metrics)

	files, err := os.ReadDir(debugDir)
	if err != nil {
		t.Fatalf("Failed to read debug directory: %v", err)
	}
	if len(files) < 4 { // at least 2 .txt and 2 .json files
		t.Fatalf("Expected at least 4 debug files, got %d", len(files))
	}

	var hasPromptTxt, hasPromptJSON, hasResponseTxt, hasResponseJSON bool
	for _, file := range files {
		name := file.Name()

		info, err := file.Info()
		if err != nil {
			t.Fatalf("Failed to get file info for %s: %v", name, err)
		}
		if perm := info.Mode().Perm(); perm != os.FileMode(0o600) {
			t.Errorf("File %s has insecure permissions %v, expected 0600", name, perm)
		}

		switch {
		case strings.HasPrefix(name, "prompt_") && strings.HasSuffix(name, ".txt"):
			hasPromptTxt = true
			content := readFile(t, filepath.Join(debugDir, name))
			assertContains(t, content, "Agent: TestAgent", "agent name")
			assertContains(t, content, "User ID: test_user_123", "user ID")
			assertContains(t, content, "You are a helpful assistant", "system message content")
			assertContains(t, content, "test_tool", "tool definition")
		case strings.HasPrefix(name, "prompt_") && strings.HasSuffix(name, ".json"):
			hasPromptJSON = true
		case strings.HasPrefix(name, "response_") && strings.HasSuffix(name, ".txt"):
			hasResponseTxt = true
			content := readFile(t, filepath.Join(debugDir, name))
			assertContains(t, content, "Agent: TestAgent", "agent name")
			assertContains(t, content, "Duration: 1234ms", "duration")
			assertContains(t, content, "Input Tokens: 100", "input tokens")
			assertContains(t, content, "Output Tokens: 50", "output tokens")
			assertContains(t, content, "Hello! I'd be happy to help you", "response content")
			assertContains(t, content, "test_tool", "tool call")
		case strings.HasPrefix(name, "response_") && strings.HasSuffix(name, ".json"):
			hasResponseJSON = true
		}
	}

	if !hasPromptTxt {
		t.Error("Missing prompt .txt file")
	}
	if !hasPromptJSON {
		t.Error("Missing prompt .json file")
	}
	if !hasResponseTxt {
		t.Error("Missing response .txt file")
	}
	if !hasResponseJSON {
		t.Error("Missing response .json file")
	}
}

func TestDebugDisabled(t *testing.T) {
	os.Unsetenv("AGENT_DEBUG_PROMPTS")
	debugEnabledOnce = sync.Once{}

	if IsDebugEnabled() {
		t.Fatal("Debug mode should be disabled")
	}

	os.RemoveAll(debugDir)

	ctx := context.Background()
	request := ModelCallRequest{
		Messages: []ModelMessage{
			{
				Role:  base.RoleSystem,
				Parts: []base.ContentPart{base.TextPart{Text: "Test"}},
			},
		},
	}

	LogPromptDebug(ctx, "TestAgent", "test-model", request)

	if _, err := os.Stat(debugDir); !os.IsNotExist(err) {
		t.Error("Debug directory should not exist when debug is disabled")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", path, err)
	}
	return string(content)
}

func assertContains(t *testing.T, content, substr, what string) {
	t.Helper()
	if !strings.Contains(content, substr) {
		t.Errorf("debug file missing %s (%q)", what, substr)
	}
}
