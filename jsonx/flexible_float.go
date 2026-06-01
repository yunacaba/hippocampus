package jsonx

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// FlexibleFloat64 handles numeric fields that can be either strings or numbers in API responses.
// This is common when external APIs inconsistently return the same field as different types.
//
// Examples:
//   - API sometimes returns: "score": 7.5
//   - API sometimes returns: "score": "7.5"
//
// Both cases will be correctly parsed into a float64 value.
type FlexibleFloat64 struct {
	value float64
}

// UnmarshalJSON implements json.Unmarshaler for flexible float64 handling.
// It tries to parse as a number first, then falls back to string parsing.
func (f *FlexibleFloat64) UnmarshalJSON(data []byte) error {
	// Try parsing as number first (most common case)
	if err := json.Unmarshal(data, &f.value); err == nil {
		return nil
	}

	// Fall back to string parsing
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return fmt.Errorf("field must be either a number or a string representing a number")
	}

	val, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return fmt.Errorf("cannot convert string %q to float64: %w", str, err)
	}

	f.value = val
	return nil
}

// MarshalJSON implements json.Marshaler to serialize back to a number.
func (f FlexibleFloat64) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.value)
}

// Float64 returns the underlying float64 value.
func (f *FlexibleFloat64) Float64() float64 {
	return f.value
}

// String returns a string representation of the float64 value.
func (f *FlexibleFloat64) String() string {
	return fmt.Sprintf("%g", f.value)
}
