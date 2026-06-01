package jsonx

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlexibleFloat64_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
		wantErr  bool
	}{
		{
			name:     "numeric value",
			input:    "7.9",
			expected: 7.9,
			wantErr:  false,
		},
		{
			name:     "string value",
			input:    "\"7.9\"",
			expected: 7.9,
			wantErr:  false,
		},
		{
			name:     "integer value",
			input:    "5",
			expected: 5.0,
			wantErr:  false,
		},
		{
			name:     "string integer value",
			input:    "\"5\"",
			expected: 5.0,
			wantErr:  false,
		},
		{
			name:     "zero value",
			input:    "0",
			expected: 0.0,
			wantErr:  false,
		},
		{
			name:     "string zero value",
			input:    "\"0\"",
			expected: 0.0,
			wantErr:  false,
		},
		{
			name:     "negative value",
			input:    "-3.14",
			expected: -3.14,
			wantErr:  false,
		},
		{
			name:     "negative string value",
			input:    "\"-3.14\"",
			expected: -3.14,
			wantErr:  false,
		},
		{
			name:     "invalid string",
			input:    "\"invalid\"",
			expected: 0.0,
			wantErr:  true,
		},
		{
			name:     "empty string",
			input:    "\"\"",
			expected: 0.0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ff FlexibleFloat64
			err := json.Unmarshal([]byte(tt.input), &ff)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, ff.Float64())
			}
		})
	}
}

func TestFlexibleFloat64_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		expected string
	}{
		{
			name:     "positive decimal",
			value:    7.9,
			expected: "7.9",
		},
		{
			name:     "integer value",
			value:    5.0,
			expected: "5",
		},
		{
			name:     "zero value",
			value:    0.0,
			expected: "0",
		},
		{
			name:     "negative value",
			value:    -3.14,
			expected: "-3.14",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ff := FlexibleFloat64{value: tt.value}
			data, err := json.Marshal(ff)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(data))
		})
	}
}

func TestFlexibleFloat64_String(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		expected string
	}{
		{
			name:     "positive decimal",
			value:    7.9,
			expected: "7.9",
		},
		{
			name:     "integer value",
			value:    5.0,
			expected: "5",
		},
		{
			name:     "zero value",
			value:    0.0,
			expected: "0",
		},
		{
			name:     "negative value",
			value:    -3.14,
			expected: "-3.14",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ff := FlexibleFloat64{value: tt.value}
			assert.Equal(t, tt.expected, ff.String())
		})
	}
}

func TestFlexibleFloat64_InStruct(t *testing.T) {
	// Test FlexibleFloat64 within a struct context
	type TestStruct struct {
		Score1 FlexibleFloat64 `json:"score1"`
		Score2 FlexibleFloat64 `json:"score2"`
		Name   string          `json:"name"`
	}

	jsonData := `{
		"score1": "4.5",
		"score2": 7.8,
		"name": "test"
	}`

	var ts TestStruct
	err := json.Unmarshal([]byte(jsonData), &ts)
	require.NoError(t, err)

	assert.Equal(t, "test", ts.Name)
	assert.Equal(t, float64(4.5), ts.Score1.Float64())
	assert.Equal(t, float64(7.8), ts.Score2.Float64())
}

func TestFlexibleFloat64_RoundTrip(t *testing.T) {
	// Test that we can unmarshal and then marshal back
	original := `{"value": "3.14159"}`

	type TestStruct struct {
		Value FlexibleFloat64 `json:"value"`
	}

	var ts TestStruct
	err := json.Unmarshal([]byte(original), &ts)
	require.NoError(t, err)

	// Verify the value was parsed correctly
	assert.Equal(t, float64(3.14159), ts.Value.Float64())

	// Marshal back to JSON
	data, err := json.Marshal(ts)
	require.NoError(t, err)

	// Should be serialized as a number, not a string
	expected := `{"value":3.14159}`
	assert.Equal(t, expected, string(data))
}
