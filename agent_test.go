package hippocampus_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/agenttest"
	"github.com/yunacaba/hippocampus/base"
	agentData "github.com/yunacaba/hippocampus/internal/testdata"
)

func toolCall(id, name, args string) base.ModelToolCall {
	return base.ModelToolCall{
		ToolCallID:   id,
		FunctionCall: base.FunctionCall{Name: name, Arguments: args},
	}
}

func TestAgent(t *testing.T) {
	response1 := &base.ModelCallResponse{
		ToolCalls: []base.ModelToolCall{
			toolCall("1", "sheaficate", `{"sheafs": ["one", "two", "three"]}`),
		},
	}
	response2 := &base.ModelCallResponse{
		ToolCalls: []base.ModelToolCall{
			toolCall("2", "bifurcate", `{"furcates": ["one", "two", "five", "six"]}`),
		},
	}
	response3 := &base.ModelCallResponse{
		Content: "{ \"sheaf\": \"I am now in a sheaf: five, six\", \"furcate_one\": \"seven\", \"furcate_two\": \"eight\", \"extra_furcates\": [\"nine\", \"ten\"] }",
	}

	sheaficationTool := hippo.NewTool(
		"sheaficate",
		"Sheafs",
		agentData.Sheaficate,
		&agentData.SheaficationRequest{
			Sheafs: []string{"one", "two", "three"},
		},
		agentData.SampleSheaficationResponse,
	)

	bifurcationTool := hippo.NewTool(
		"bifurcate",
		"Bifurcates",
		agentData.Bifurcate,
		&agentData.BifurcationRequest{
			Furcates: []string{"one", "two", "five", "six"},
		},
		agentData.SampleBifurcationResponse,
	)

	model := agenttest.NewMockModel(
		"test_agent",
		hippo.AnthropicClaudeHaiku45,
		[]*base.ModelCallResponse{
			response1,
			response2,
			response3,
		},
	)
	modelProvider := agenttest.NewMockModelProvider()
	modelProvider.AddModel("test_agent", model)

	agent, err := hippo.NewAgentWithTemplateText(
		"You are a helpful assistant that can sheaficate and bifurcate.",
		&agentData.TestAgentRequest{},
		agentData.SampleTestAgentResponse,
	).
		SetName("test_agent").
		SetModel(modelProvider, hippo.AnthropicClaudeHaiku45).
		AddTool(sheaficationTool).
		AddTool(bifurcationTool).
		Build()
	require.NoError(t, err)
	response, details, err := agent.ExecuteWithDetails(
		context.Background(),
		&agentData.TestAgentRequest{
			Sheafs:   []string{"one", "two", "three"},
			Furcates: []string{"one", "two", "five", "six"},
		},
		nil,
	)
	require.NoError(t, err)
	assert.Equal(t, "I am now in a sheaf: five, six", response.Sheaf)
	assert.Equal(t, "seven", response.FurcateOne)
	assert.Equal(t, "eight", response.FurcateTwo)
	assert.Equal(t, []string{"nine", "ten"}, response.ExtraFurcates)

	assert.Equal(t, 3, len(details.ModelResponses))
}

// TestAgent_RecordedRequestsNotMutated guards against the slice-aliasing bug:
// a request recorded in details.ModelRequests must not be mutated by appends to
// the agent's working message slice in a later iteration. With tools present,
// each recorded request must end with that iteration's guidance message.
func TestAgent_RecordedRequestsNotMutated(t *testing.T) {
	type TestRequest struct {
		Message string `json:"message"`
	}
	type TestResponse struct {
		Result string `json:"result"`
	}

	echoTool := hippo.NewTool(
		"echo_tool",
		"Echoes back the input message",
		func(ctx context.Context, input *TestRequest, invocationId string) (*TestResponse, error) {
			return &TestResponse{Result: "echo: " + input.Message}, nil
		},
		&TestRequest{},
		&TestResponse{},
	)

	// Two tool-calling iterations, then a final answer. The second tool-calling
	// iteration is the one that previously corrupted the first recorded request.
	mockModel := agenttest.NewMockModel(
		"test_agent",
		hippo.AnthropicClaudeHaiku45,
		[]*base.ModelCallResponse{
			{ToolCalls: []base.ModelToolCall{toolCall("1", "echo_tool", `{"message": "one"}`)}},
			{ToolCalls: []base.ModelToolCall{toolCall("2", "echo_tool", `{"message": "two"}`)}},
			{Content: `{"result": "done"}`},
		},
	)
	mockProvider := agenttest.NewMockModelProvider()
	mockProvider.AddModel("test_agent", mockModel)

	agent, err := hippo.NewAgentWithTemplateText(
		"Echo agent.",
		&TestRequest{},
		&TestResponse{},
	).
		SetName("test_agent").
		SetModel(mockProvider, hippo.AnthropicClaudeHaiku45).
		AddTool(echoTool).
		Build()
	require.NoError(t, err)

	_, details, err := agent.ExecuteWithDetails(context.Background(), &TestRequest{Message: "hi"}, nil)
	require.NoError(t, err)
	require.Equal(t, 3, len(details.ModelRequests))

	// Each recorded request (tools present) must end with its iteration guidance.
	for i, req := range details.ModelRequests {
		last := req.Messages[len(req.Messages)-1]
		tp, ok := last.Parts[0].(base.TextPart)
		if !ok {
			t.Fatalf("request %d: last message is %T, want a TextPart iteration guidance", i, last.Parts[0])
		}
		assert.Contains(t, tp.Text, "Iteration", "request %d last message should be iteration guidance, got %q", i, tp.Text)
	}
}

func TestAgent_ObserverCallbacks(t *testing.T) {
	type TestRequest struct {
		Message string `json:"message"`
	}
	type TestResponse struct {
		Result string `json:"result"`
	}

	echoTool := hippo.NewTool(
		"echo_tool",
		"Echoes back the input message",
		func(ctx context.Context, input *TestRequest, invocationId string) (*TestResponse, error) {
			return &TestResponse{Result: "echo: " + input.Message}, nil
		},
		&TestRequest{},
		&TestResponse{},
	)

	observer := agenttest.NewMockAgentObserver[*TestResponse]()

	mockModel := agenttest.NewMockModel(
		"test_agent",
		hippo.AnthropicClaudeHaiku45,
		[]*base.ModelCallResponse{
			{
				Content: "",
				ToolCalls: []base.ModelToolCall{
					toolCall("1", "echo_tool", `{"message": "hello world"}`),
				},
			},
			{
				Content:   `{"result": "final response"}`,
				ToolCalls: nil,
			},
		},
	)

	mockProvider := agenttest.NewMockModelProvider()
	mockProvider.AddModel("test_agent", mockModel)

	agent, err := hippo.NewAgentWithTemplateText(
		"You are a test agent that can echo messages.",
		&TestRequest{},
		&TestResponse{},
	).
		SetName("test_agent").
		SetModel(mockProvider, hippo.AnthropicClaudeHaiku45).
		AddTool(echoTool).
		SetObserver(observer).
		Build()
	require.NoError(t, err)

	resp, details, err := agent.ExecuteWithDetails(context.Background(), &TestRequest{Message: "hello world"}, nil)
	require.NoError(t, err)

	assert.Equal(t, "final response", resp.Result)
	assert.Equal(t, 2, len(details.ModelResponses))

	expectedCalls := []string{
		"OnLLMCall",
		"OnToolCall: echo_tool ID: 1",
		"OnToolCallResponse: echo_tool ID: 1",
		"OnLLMCall",
		"OnLLMResponse",
	}
	assert.Equal(t, expectedCalls, observer.CallSequence, "Observer calls should match expected sequence")

	assert.Equal(t, 1, len(observer.ToolCalls))
	assert.Equal(t, "echo_tool", observer.ToolCalls[0].FunctionCall.Name)
	assert.Equal(t, `{"message": "hello world"}`, observer.ToolCalls[0].FunctionCall.Arguments)

	assert.Equal(t, 1, len(observer.ToolCallResponses))
	assert.Equal(t, "1", observer.ToolCallResponses[0].ToolCallID)
	assert.Equal(t, "echo_tool", observer.ToolCallResponses[0].Name)
	assert.Contains(t, observer.ToolCallResponses[0].Content, "echo: hello world")
}

func TestAgent_ToolDelegate_AcceptAll(t *testing.T) {
	type TestRequest struct {
		Message string `json:"message"`
	}
	type TestResponse struct {
		Result string `json:"result"`
	}

	echoTool := hippo.NewTool(
		"echo_tool",
		"Echoes back the input message",
		func(ctx context.Context, input *TestRequest, invocationId string) (*TestResponse, error) {
			return &TestResponse{Result: "echo: " + input.Message}, nil
		},
		&TestRequest{},
		&TestResponse{},
	)

	acceptAllDelegate := &agenttest.MockToolDelegate{
		RewriteToolCallsFunc: func(
			ctx context.Context,
			toolCalls []*base.ModelToolCall,
		) {
			for _, tc := range toolCalls {
				tc.DelegateAction = base.ToolDelegateActionAccepted
			}
		},
	}

	mockModel := agenttest.NewMockModel(
		"test_agent",
		hippo.AnthropicClaudeHaiku45,
		[]*base.ModelCallResponse{
			{
				Content: "",
				ToolCalls: []base.ModelToolCall{
					toolCall("1", "echo_tool", `{"message": "hello world"}`),
				},
			},
			{
				Content:   `{"result": "final response"}`,
				ToolCalls: nil,
			},
		},
	)

	mockProvider := agenttest.NewMockModelProvider()
	mockProvider.AddModel("test_agent", mockModel)

	agent, err := hippo.NewAgentWithTemplateText(
		"You are a test agent that can echo messages.",
		&TestRequest{},
		&TestResponse{},
	).
		SetName("test_agent").
		SetModel(mockProvider, hippo.AnthropicClaudeHaiku45).
		AddTool(echoTool).
		SetToolDelegate(acceptAllDelegate).
		Build()
	require.NoError(t, err)

	resp, details, err := agent.ExecuteWithDetails(context.Background(), &TestRequest{Message: "hello world"}, nil)
	require.NoError(t, err)

	assert.Equal(t, "final response", resp.Result)
	assert.Equal(t, 2, len(details.ModelResponses))

	assert.Equal(t, 1, acceptAllDelegate.RewriteToolCallsCallCount)
	assert.Equal(t, 1, len(acceptAllDelegate.LastToolCalls))
	assert.Equal(t, "echo_tool", acceptAllDelegate.LastToolCalls[0].FunctionCall.Name)
}

func TestAgent_ToolDelegate_RejectAll(t *testing.T) {
	type TestRequest struct {
		Message string `json:"message"`
	}
	type TestResponse struct {
		Result string `json:"result"`
	}

	echoTool := hippo.NewTool(
		"echo_tool",
		"Echoes back the input message",
		func(ctx context.Context, input *TestRequest, invocationId string) (*TestResponse, error) {
			return &TestResponse{Result: "echo: " + input.Message}, nil
		},
		&TestRequest{},
		&TestResponse{},
	)

	rejectAllDelegate := &agenttest.MockToolDelegate{
		RewriteToolCallsFunc: func(
			ctx context.Context,
			toolCalls []*base.ModelToolCall,
		) {
			for _, tc := range toolCalls {
				tc.RejectWithReason("tool calls not allowed in this context")
			}
		},
	}

	mockModel := agenttest.NewMockModel(
		"test_agent",
		hippo.AnthropicClaudeHaiku45,
		[]*base.ModelCallResponse{
			{
				Content: "",
				ToolCalls: []base.ModelToolCall{
					toolCall("1", "echo_tool", `{"message": "hello world"}`),
				},
			},
			{
				Content:   `{"result": "final response"}`,
				ToolCalls: nil,
			},
		},
	)

	mockProvider := agenttest.NewMockModelProvider()
	mockProvider.AddModel("test_agent", mockModel)

	mockAgentObserver := agenttest.NewMockAgentObserver[*TestResponse]()

	agent, err := hippo.NewAgentWithTemplateText(
		"You are a test agent that can echo messages.",
		&TestRequest{},
		&TestResponse{},
	).
		SetName("test_agent").
		SetModel(mockProvider, hippo.AnthropicClaudeHaiku45).
		AddTool(echoTool).
		SetToolDelegate(rejectAllDelegate).
		SetObserver(mockAgentObserver).
		Build()
	require.NoError(t, err)

	resp, details, err := agent.ExecuteWithDetails(context.Background(), &TestRequest{Message: "hello world"}, nil)
	require.NoError(t, err)

	// Rejected tool calls should still cause iteration because
	// they generate error responses that get sent back to LLM.
	assert.Equal(t, "final response", resp.Result)
	assert.Equal(t, 2, len(details.ModelResponses))

	assert.Equal(t, 1, rejectAllDelegate.RewriteToolCallsCallCount)
	assert.Equal(t, 1, len(rejectAllDelegate.LastToolCalls))
	assert.Equal(t, base.ToolDelegateActionRejected, rejectAllDelegate.LastToolCalls[0].DelegateAction)

	assert.Equal(t, 1, len(mockAgentObserver.ToolCallErrors))
	assert.Contains(t, mockAgentObserver.ToolCallErrors[0].Error(), "tool call 1 rejected: tool calls not allowed in this context")
}

func TestAgent_ToolDelegate_ModifyToolCalls(t *testing.T) {
	type TestRequest struct {
		Message string `json:"message"`
	}
	type TestResponse struct {
		Result string `json:"result"`
	}

	echoTool := hippo.NewTool(
		"echo_tool",
		"Echoes back the input message",
		func(ctx context.Context, input *TestRequest, invocationId string) (*TestResponse, error) {
			return &TestResponse{Result: "echo: " + input.Message}, nil
		},
		&TestRequest{},
		&TestResponse{},
	)

	modifyDelegate := &agenttest.MockToolDelegate{
		RewriteToolCallsFunc: func(
			ctx context.Context,
			toolCalls []*base.ModelToolCall,
		) {
			for _, tc := range toolCalls {
				tc.DelegateAction = base.ToolDelegateActionAccepted
				tc.FunctionCall.Arguments = `{"message": "modified by delegate"}`
			}
		},
	}

	mockModel := agenttest.NewMockModel(
		"test_agent",
		hippo.AnthropicClaudeHaiku45,
		[]*base.ModelCallResponse{
			{
				Content: "",
				ToolCalls: []base.ModelToolCall{
					toolCall("1", "echo_tool", `{"message": "hello world"}`),
				},
			},
			{
				Content:   `{"result": "final response"}`,
				ToolCalls: nil,
			},
		},
	)

	mockProvider := agenttest.NewMockModelProvider()
	mockProvider.AddModel("test_agent", mockModel)

	agent, err := hippo.NewAgentWithTemplateText(
		"You are a test agent that can echo messages.",
		&TestRequest{},
		&TestResponse{},
	).
		SetName("test_agent").
		SetModel(mockProvider, hippo.AnthropicClaudeHaiku45).
		AddTool(echoTool).
		SetToolDelegate(modifyDelegate).
		Build()
	require.NoError(t, err)

	resp, details, err := agent.ExecuteWithDetails(context.Background(), &TestRequest{Message: "hello world"}, nil)
	require.NoError(t, err)

	assert.Equal(t, "final response", resp.Result)
	assert.Equal(t, 2, len(details.ModelResponses))

	assert.Equal(t, 1, modifyDelegate.RewriteToolCallsCallCount)
	assert.Equal(t, 1, len(modifyDelegate.LastToolCalls))
	assert.Equal(t, `{"message": "modified by delegate"}`, modifyDelegate.LastToolCalls[0].FunctionCall.Arguments)
	assert.Equal(t, base.ToolDelegateActionAccepted, modifyDelegate.LastToolCalls[0].DelegateAction)
}

func TestAgent_ToolDelegate_UnhandledToolError(t *testing.T) {
	type TestRequest struct {
		Message string `json:"message"`
	}
	type TestResponse struct {
		Result string `json:"result"`
	}

	echoTool := hippo.NewTool(
		"echo_tool",
		"Echoes back the input message",
		func(ctx context.Context, input *TestRequest, invocationId string) (*TestResponse, error) {
			return &TestResponse{Result: "echo: " + input.Message}, nil
		},
		&TestRequest{},
		&TestResponse{},
	)

	// Delegate that only handles the first tool call, leaving the second pending.
	partialDelegate := &agenttest.MockToolDelegate{
		RewriteToolCallsFunc: func(
			ctx context.Context,
			toolCalls []*base.ModelToolCall,
		) {
			if len(toolCalls) > 0 {
				toolCalls[0].DelegateAction = base.ToolDelegateActionAccepted
			}
		},
	}

	mockModel := agenttest.NewMockModel(
		"test_agent",
		hippo.AnthropicClaudeHaiku45,
		[]*base.ModelCallResponse{
			{
				Content: "",
				ToolCalls: []base.ModelToolCall{
					toolCall("1", "echo_tool", `{"message": "first call"}`),
					toolCall("2", "echo_tool", `{"message": "second call"}`),
				},
			},
			{
				Content:   `{"result": "final response"}`,
				ToolCalls: nil,
			},
		},
	)

	mockProvider := agenttest.NewMockModelProvider()
	mockProvider.AddModel("test_agent", mockModel)

	mockAgentObserver := agenttest.NewMockAgentObserver[*TestResponse]()

	agent, err := hippo.NewAgentWithTemplateText(
		"You are a test agent that can echo messages.",
		&TestRequest{},
		&TestResponse{},
	).
		SetName("test_agent").
		SetModel(mockProvider, hippo.AnthropicClaudeHaiku45).
		AddTool(echoTool).
		SetToolDelegate(partialDelegate).
		SetObserver(mockAgentObserver).
		Build()
	require.NoError(t, err)

	resp, details, err := agent.ExecuteWithDetails(context.Background(), &TestRequest{Message: "hello world"}, nil)
	require.NoError(t, err)

	// Pending tool calls are automatically rejected with internal error,
	// allowing the agent to continue.
	assert.Equal(t, "final response", resp.Result)
	assert.Equal(t, 2, len(details.ModelResponses))

	assert.Equal(t, 1, len(mockAgentObserver.ToolCallErrors))
	assert.Contains(t, mockAgentObserver.ToolCallErrors[0].Error(), "tool call 2 rejected: internal error")

	assert.Equal(t, 1, len(mockAgentObserver.ToolCallResponses))
	assert.Equal(t, "echo_tool", mockAgentObserver.ToolCallResponses[0].Name)
	assert.Equal(t, `{"result":"echo: first call"}`, mockAgentObserver.ToolCallResponses[0].Content)

	assert.Equal(t, 2, len(details.ModelRequests))
	request := details.ModelRequests[1]
	assert.Equal(t, 6, len(request.Messages))

	// The second round trip should have both tool calls,
	// one accepted and one rejected. Inspect owned content parts.
	foundToolCall := false
	foundToolCallResponse := false
	for _, message := range request.Messages {
		parts := message.Parts
		if len(parts) > 0 {
			part := parts[0]
			if tc, ok := part.(base.ToolCallPart); ok {
				assert.Equal(t, "echo_tool", tc.Name)
				foundToolCall = true
			} else if tr, ok := part.(base.ToolResultPart); ok {
				assert.Equal(t, "echo_tool", tr.Name)
				if tr.Content == `Error: tool call 2 rejected: internal error` {
					foundToolCallResponse = true
				}
			}
		}
	}
	assert.True(t, foundToolCall)
	assert.True(t, foundToolCallResponse)

	assert.Equal(t, 1, partialDelegate.RewriteToolCallsCallCount)
	assert.Equal(t, 2, len(partialDelegate.LastToolCalls))

	assert.Equal(t, base.ToolDelegateActionAccepted, partialDelegate.LastToolCalls[0].DelegateAction)
	assert.Equal(t, base.ToolDelegateActionRejected, partialDelegate.LastToolCalls[1].DelegateAction)
}
