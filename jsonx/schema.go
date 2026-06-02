package jsonx

import (
	sysjson "encoding/json"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
)

// Schema generation infers a JSON Schema from a Go type using
// github.com/google/jsonschema-go (the package the Go MCP SDK standardizes on).
// A field is treated as required unless it is tagged `omitempty`/`omitzero`;
// property names come from the `json` tag and descriptions from a `jsonschema`
// tag. The argument's value is unused beyond its type — pointers are
// dereferenced — so passing a zero/sample instance is fine.

// schemaForValue infers a schema for the dynamic type of value, dereferencing
// pointer types.
func schemaForValue(value any) (*jsonschema.Schema, error) {
	t := reflect.TypeOf(value)
	if t == nil {
		// Untyped nil — nothing to reflect; return an empty schema.
		return &jsonschema.Schema{}, nil
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return jsonschema.ForType(t, nil)
}

// SchemaBytes returns the JSON Schema for value's type as JSON bytes.
func SchemaBytes[T any](value T) ([]byte, error) {
	schema, err := schemaForValue(value)
	if err != nil {
		return nil, err
	}
	return sysjson.Marshal(schema)
}

// SchemaString returns the JSON Schema for value's type as a JSON string.
func SchemaString[T any](value T) (string, error) {
	schemaBytes, err := SchemaBytes(value)
	if err != nil {
		return "", err
	}
	return string(schemaBytes), nil
}

// SchemaMap returns the JSON Schema for value's type as a decoded map, suitable
// for use as an LLM tool's parameter schema.
func SchemaMap[T any](value T) (map[string]interface{}, error) {
	schemaBytes, err := SchemaBytes(value)
	if err != nil {
		return nil, err
	}

	var schemaMap map[string]interface{}
	if err := sysjson.Unmarshal(schemaBytes, &schemaMap); err != nil {
		return nil, err
	}

	return schemaMap, nil
}
