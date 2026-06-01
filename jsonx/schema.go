package jsonx

import (
	sysjson "encoding/json"

	"github.com/swaggest/jsonschema-go"
)

func SchemaBytes[T any](value T) ([]byte, error) {
	reflector := jsonschema.Reflector{}
	schema, err := reflector.Reflect(value)
	if err != nil {
		return nil, err
	}

	schemaBytes, err := sysjson.Marshal(schema)
	if err != nil {
		return nil, err
	}

	return schemaBytes, nil
}

func SchemaString[T any](value T) (string, error) {
	schemaBytes, err := SchemaBytes(value)
	if err != nil {
		return "", err
	}
	return string(schemaBytes), nil
}

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
