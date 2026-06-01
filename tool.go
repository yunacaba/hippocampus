package hippocampus

import (
	"context"
	"reflect"

	"github.com/yunacaba/hippocampus/base"
	"github.com/yunacaba/hippocampus/jsonx"
)

// ToolFunction represents a tool function with type-safe input and output
// that returns a result immediately.
type ToolFunction[I any, O any] func(
	ctx context.Context,
	input I,
	invocationId string,
) (O, error)

// AnyTool is a type-erased tool.
type AnyTool interface {
	Name() string
	Description() string
	ParametersSchema() map[string]interface{}
	Invoke(
		ctx context.Context,
		jsonParams string,
		invocationId string,
	) (string, error)
	toToolSpec() base.ToolSpec
}

// Tool represents an LLM-callable tool with type-safe arguments and results.
type Tool[I any, O any] struct {
	name         string
	description  string
	function     ToolFunction[I, O]
	argType      reflect.Type // Type of the argument struct
	resultType   reflect.Type // Type of the result struct
	sampleInput  I            // Sample input for schema generation
	sampleOutput O            // Sample output for documentation

	// schema is the JSON Schema for the input type, computed once at
	// construction (sampleInput is immutable) rather than on every call.
	schema map[string]interface{}
}

// Ensure Tool implements AnyTool (with any types for the generic).
var _ AnyTool = (*Tool[any, any])(nil)

// NewTool creates a new tool with type-safe input and output.
func NewTool[I any, O any](
	name string,
	description string,
	function ToolFunction[I, O],
	sampleInput I,
	sampleOutput O,
) *Tool[I, O] {
	return newTool(name, description, function, sampleInput, sampleOutput)
}

func newTool[I any, O any](
	name string,
	description string,
	asyncWrapper ToolFunction[I, O],
	sampleInput I,
	sampleOutput O,
) *Tool[I, O] {
	argType := reflect.TypeOf(sampleInput)
	resultType := reflect.TypeOf(sampleOutput)

	// Handle pointer types for reflection
	if argType != nil && argType.Kind() == reflect.Ptr {
		argType = argType.Elem()
	}
	if resultType != nil && resultType.Kind() == reflect.Ptr {
		resultType = resultType.Elem()
	}

	schema, err := jsonx.SchemaMap(sampleInput)
	if err != nil {
		schema = map[string]interface{}{"type": "object"}
	}

	return &Tool[I, O]{
		name:         name,
		description:  description,
		function:     asyncWrapper,
		argType:      argType,
		resultType:   resultType,
		sampleInput:  sampleInput,
		sampleOutput: sampleOutput,
		schema:       schema,
	}
}

func (t *Tool[I, O]) Name() string {
	return t.name
}

func (t *Tool[I, O]) Description() string {
	return t.description
}

// ParametersSchema returns the JSON Schema for the tool's parameters. The
// schema is computed once at construction and reused.
func (t *Tool[I, O]) ParametersSchema() map[string]interface{} {
	return t.schema
}

// Invoke calls the tool's function with the provided JSON parameters.
func (t *Tool[I, O]) Invoke(
	ctx context.Context,
	jsonParams string,
	invocationId string,
) (string, error) {
	input, err := jsonx.Deserialize[I](jsonParams, t.sampleInput)
	if err != nil {
		return "", err
	}

	result, err := t.function(ctx, input, invocationId)
	if err != nil {
		return "", err
	}

	serializedResult, err := jsonx.SerializeToString(result)
	if err != nil {
		return "", err
	}

	return serializedResult, nil
}

// toToolSpec converts the tool to a provider-neutral ToolSpec for model calls.
func (t *Tool[I, O]) toToolSpec() base.ToolSpec {
	return base.ToolSpec{
		Name:        t.name,
		Description: t.description,
		Schema:      t.ParametersSchema(),
	}
}
