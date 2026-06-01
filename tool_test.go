package hippocampus_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	hippo "github.com/yunacaba/hippocampus"
)

// Test struct for tool testing
type TestInput struct {
	Name   string `json:"name"`
	Age    int    `json:"age"`
	Active bool   `json:"active"`
}

type TestOutput struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
}

func TestTool_Sync(t *testing.T) {
	toolFunc := func(ctx context.Context, input *TestInput, invocationId string) (*TestOutput, error) {
		return &TestOutput{
			Message: "Hello " + input.Name,
			Success: input.Active,
		}, nil
	}

	tool := hippo.NewTool(
		"test_tool",
		"A test tool for validation",
		toolFunc,
		&TestInput{},
		&TestOutput{},
	)

	assert.Equal(t, "test_tool", tool.Name())
	assert.Equal(t, "A test tool for validation", tool.Description())

	schema := tool.ParametersSchema()
	assert.Contains(t, schema, "type")
	assert.Equal(t, "object", schema["type"])

	jsonInput := `{"name": "Alice", "age": 30, "active": true}`
	result, err := tool.Invoke(context.Background(), jsonInput, "test_invocation_id")
	require.NoError(t, err)

	assert.Contains(t, result, "Hello Alice")
	assert.Contains(t, result, "success")
}

// TestTool_ProtocolBufferHandling exercises the jsonx protojson path by using a
// proto.Message (structpb.Struct) as both the tool input and output type.
func TestTool_ProtocolBufferHandling(t *testing.T) {
	toolFunc := func(
		ctx context.Context,
		input *structpb.Struct,
		invocationId string,
	) (*structpb.Struct, error) {
		displayName := input.GetFields()["display_name"].GetStringValue()
		return structpb.NewStruct(map[string]any{
			"traveler_id":  "test_traveler_id",
			"display_name": displayName,
		})
	}

	tool := hippo.NewTool(
		"proto_tool",
		"A tool that accepts and returns a protobuf message",
		toolFunc,
		&structpb.Struct{},
		&structpb.Struct{},
	)

	jsonInput := `{"user_id": "test_user_id", "display_name": "The Yunapotamus"}`
	result, err := tool.Invoke(context.Background(), jsonInput, "test_invocation_id")
	require.NoError(t, err)
	assert.Contains(t, result, "test_traveler_id")
	assert.Contains(t, result, "The Yunapotamus")
}

func TestTool_ErrorHandling(t *testing.T) {
	toolFunc := func(ctx context.Context, input *TestInput, invocationId string) (*TestOutput, error) {
		if input.Name == "" {
			return nil, assert.AnError
		}
		return &TestOutput{Message: "OK", Success: true}, nil
	}

	tool := hippo.NewTool(
		"error_tool",
		"A tool that might error",
		toolFunc,
		&TestInput{},
		&TestOutput{},
	)

	jsonInput := `{"name": "", "age": 25, "active": true}`
	_, err := tool.Invoke(context.Background(), jsonInput, "test_invocation_id")
	require.Error(t, err)

	jsonInput = `{"name": "Bob", "age": 25, "active": true}`
	result, err := tool.Invoke(context.Background(), jsonInput, "test_invocation_id")
	require.NoError(t, err)
	assert.Contains(t, result, "OK")
}
