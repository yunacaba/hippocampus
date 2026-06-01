// Package jsonx provides JSON serialization and deserialization utilities.
//
// This package offers two main deserialization approaches:
//
//  1. Deserialize[T] - Type-safe deserialization that creates fresh instances
//     Use when you have compile-time type information and want to avoid mutation
//
//  2. DeserializeAny - Traditional in-place deserialization for type-erased objects
//     Use when you have runtime-only type information or need in-place mutation
package jsonx

import (
	"fmt"
	"reflect"

	sysjson "encoding/json"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Deserialize deserializes JSON into a new instance of type T, using the prototype
// to determine the target type. The prototype is NEVER mutated; a fresh instance
// is always created and returned.
//
// When to use Deserialize:
// - When you want type-safe deserialization with compile-time type checking
// - When you have a prototype/sample instance that should not be mutated
// - When the target type T is known at compile time (e.g., in generic functions)
// - When you want to ensure the prototype remains unchanged for reuse
//
// Example:
//
//	type User struct { Name string `json:"name"` }
//	prototype := &User{}
//	user, err := Deserialize(`{"name": "John"}`, prototype)
//
// `prototype` is unchanged, `user` is a new instance
//
// Type safety: The generic type parameter T ensures compile-time type safety.
// The returned value is guaranteed to be of type T.
func Deserialize[T any](jsonString string, prototype T) (T, error) {
	var inputPtr interface{}
	prototypeType := reflect.TypeOf(prototype)

	// Always create a fresh instance to avoid mutating the prototype
	if prototypeType.Kind() == reflect.Ptr {
		// If prototype is a pointer, create a new instance of the pointed-to type
		elementType := prototypeType.Elem()
		inputPtr = reflect.New(elementType).Interface()
	} else {
		// If prototype is not a pointer, create a new pointer to a new instance
		inputPtr = reflect.New(prototypeType).Interface()
	}

	// Special case for protobuf messages
	if protoMsg, ok := inputPtr.(proto.Message); ok {
		if err := protojson.Unmarshal([]byte(jsonString), protoMsg); err != nil {
			var zero T
			return zero, fmt.Errorf("failed to parse protobuf arguments: %w", err)
		}
		return inputPtr.(T), nil
	}

	if err := sysjson.Unmarshal([]byte(jsonString), inputPtr); err != nil {
		var zero T
		return zero, fmt.Errorf("failed to parse arguments: %w", err)
	}
	return inputPtr.(T), nil
}

// DeserializeAny deserializes JSON directly into the provided pointer, mutating
// the target object in-place. This is the traditional approach similar to
// json.Unmarshal.
//
// When to use DeserializeAny:
// - When you have a type-erased object (any/interface{}) at runtime
// - When you want in-place mutation for performance (no allocation of new instance)
// - When working with existing allocated objects that should be populated
// - When the target type is only known at runtime, not compile time
// - When integrating with APIs that expect traditional unmarshaling behavior
//
// Requirements:
// - The result parameter MUST be a pointer to the target object
// - The pointed-to object will be mutated in-place
// - No compile-time type safety - runtime type must match JSON structure
//
// Example:
//
//	var user User
//	err := DeserializeAny(`{"name": "John"}`, &user)
//
// `user` is now populated with the JSON data
//
// Type safety: Since `result` is type-erased (any), there's no compile-time
// type checking. The caller is responsible for ensuring the runtime type
// matches the expected JSON structure.
//
// Note: DeserializeAny is a low-level function that should be used with caution.
// It's generally better to use Deserialize[T] when possible, as it provides
// compile-time type safety and immutability guarantees.
func DeserializeAny(jsonString string, result any) error {
	// Caller must pass a pointer to the target object
	if reflect.TypeOf(result).Kind() != reflect.Ptr {
		return fmt.Errorf("result must be a pointer")
	}

	// Special case for protobuf messages
	if protoMsg, ok := result.(proto.Message); ok {
		if err := protojson.Unmarshal([]byte(jsonString), protoMsg); err != nil {
			return fmt.Errorf("failed to parse protobuf arguments: %w", err)
		}
		return nil
	}

	if err := sysjson.Unmarshal([]byte(jsonString), result); err != nil {
		return fmt.Errorf("failed to parse arguments: %w", err)
	}
	return nil
}

func SerializeToBytes(value any) ([]byte, error) {
	// Special case for protobuf messages
	if protoMsg, ok := value.(proto.Message); ok {
		bytes, err := protojson.Marshal(protoMsg)
		if err != nil {
			return nil, err
		}
		return bytes, nil
	}

	bytes, err := sysjson.Marshal(value)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func SerializeToString(value any) (string, error) {
	bytes, err := SerializeToBytes(value)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func SerializeToMap(value any) (map[string]interface{}, error) {
	bytes, err := SerializeToBytes(value)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := sysjson.Unmarshal(bytes, &result); err != nil {
		return nil, err
	}
	return result, nil
}
