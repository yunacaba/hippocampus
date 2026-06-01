package jsonx

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlexibleTime_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Time
		wantErr  bool
	}{
		{
			name:     "RFC3339 format",
			input:    `"2025-09-14T15:30:00Z"`,
			expected: time.Date(2025, 9, 14, 15, 30, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "RFC3339 format with offset",
			input:    `"2025-09-14T15:30:00-07:00"`,
			expected: time.Date(2025, 9, 14, 15, 30, 0, 0, time.FixedZone("-07:00", -7*3600)),
			wantErr:  false,
		},
		{
			name:     "date only format",
			input:    `"2025-09-14"`,
			expected: time.Date(2025, 9, 14, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "datetime without timezone",
			input:    `"2025-09-14T15:30:00"`,
			expected: time.Date(2025, 9, 14, 15, 30, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "native time object (RFC3339)",
			input:    `"2025-09-14T15:30:00Z"`,
			expected: time.Date(2025, 9, 14, 15, 30, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "leap year date",
			input:    `"2024-02-29"`,
			expected: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "end of year",
			input:    `"2025-12-31T23:59:59Z"`,
			expected: time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "invalid date format",
			input:    `"invalid-date"`,
			expected: time.Time{},
			wantErr:  true,
		},
		{
			name:     "empty string",
			input:    `""`,
			expected: time.Time{},
			wantErr:  true,
		},
		{
			name:     "invalid json",
			input:    `invalid`,
			expected: time.Time{},
			wantErr:  true,
		},
		{
			name:     "number input (invalid)",
			input:    `1694707800`,
			expected: time.Time{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ft FlexibleTime
			err := json.Unmarshal([]byte(tt.input), &ft)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.True(t, tt.expected.Equal(ft.Time()),
					"Expected %v, got %v", tt.expected, ft.Time())
			}
		})
	}
}

func TestFlexibleTime_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "UTC time",
			time:     time.Date(2025, 9, 14, 15, 30, 0, 0, time.UTC),
			expected: `"2025-09-14T15:30:00Z"`,
		},
		{
			name:     "time with offset",
			time:     time.Date(2025, 9, 14, 15, 30, 0, 0, time.FixedZone("-07:00", -7*3600)),
			expected: `"2025-09-14T15:30:00-07:00"`,
		},
		{
			name:     "midnight UTC",
			time:     time.Date(2025, 9, 14, 0, 0, 0, 0, time.UTC),
			expected: `"2025-09-14T00:00:00Z"`,
		},
		{
			name:     "zero time",
			time:     time.Time{},
			expected: `"0001-01-01T00:00:00Z"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ft := FlexibleTime{value: tt.time}
			data, err := json.Marshal(ft)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(data))
		})
	}
}

func TestFlexibleTime_String(t *testing.T) {
	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "UTC time",
			time:     time.Date(2025, 9, 14, 15, 30, 0, 0, time.UTC),
			expected: "2025-09-14T15:30:00Z",
		},
		{
			name:     "time with offset",
			time:     time.Date(2025, 9, 14, 15, 30, 0, 0, time.FixedZone("-07:00", -7*3600)),
			expected: "2025-09-14T15:30:00-07:00",
		},
		{
			name:     "zero time",
			time:     time.Time{},
			expected: "0001-01-01T00:00:00Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ft := FlexibleTime{value: tt.time}
			assert.Equal(t, tt.expected, ft.String())
		})
	}
}

func TestFlexibleTime_IsZero(t *testing.T) {
	tests := []struct {
		name     string
		time     time.Time
		expected bool
	}{
		{
			name:     "zero time",
			time:     time.Time{},
			expected: true,
		},
		{
			name:     "non-zero time",
			time:     time.Date(2025, 9, 14, 15, 30, 0, 0, time.UTC),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ft := FlexibleTime{value: tt.time}
			assert.Equal(t, tt.expected, ft.IsZero())
		})
	}
}

func TestFlexibleTime_InStruct(t *testing.T) {
	// Test FlexibleTime within a struct context
	type TestStruct struct {
		DepartureTime FlexibleTime `json:"departure_time"`
		ArrivalTime   FlexibleTime `json:"arrival_time"`
		FlightNumber  string       `json:"flight_number"`
	}

	jsonData := `{
		"departure_time": "2025-09-14",
		"arrival_time": "2025-09-14T15:30:00Z",
		"flight_number": "AA123"
	}`

	var ts TestStruct
	err := json.Unmarshal([]byte(jsonData), &ts)
	require.NoError(t, err)

	assert.Equal(t, "AA123", ts.FlightNumber)
	assert.Equal(t, time.Date(2025, 9, 14, 0, 0, 0, 0, time.UTC), ts.DepartureTime.Time())
	assert.Equal(t, time.Date(2025, 9, 14, 15, 30, 0, 0, time.UTC), ts.ArrivalTime.Time())
}

func TestFlexibleTime_RoundTrip(t *testing.T) {
	// Test that we can unmarshal and then marshal back
	original := `{"departure": "2025-09-14"}`

	type TestStruct struct {
		Departure FlexibleTime `json:"departure"`
	}

	var ts TestStruct
	err := json.Unmarshal([]byte(original), &ts)
	require.NoError(t, err)

	// Verify the time was parsed correctly
	expected := time.Date(2025, 9, 14, 0, 0, 0, 0, time.UTC)
	assert.True(t, expected.Equal(ts.Departure.Time()))
	assert.False(t, ts.Departure.IsZero())

	// Marshal back to JSON
	data, err := json.Marshal(ts)
	require.NoError(t, err)

	// Should be serialized as RFC3339 format
	expectedJSON := `{"departure":"2025-09-14T00:00:00Z"}`
	assert.Equal(t, expectedJSON, string(data))
}

func TestFlexibleTime_AmadeusExamples(t *testing.T) {
	// Test specific examples that would come from Amadeus API
	tests := []struct {
		name     string
		input    string
		expected time.Time
	}{
		{
			name:     "Amadeus departure time (date only)",
			input:    `"2025-09-14"`,
			expected: time.Date(2025, 9, 14, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Amadeus departure time (RFC3339)",
			input:    `"2025-09-14T15:30:00Z"`,
			expected: time.Date(2025, 9, 14, 15, 30, 0, 0, time.UTC),
		},
		{
			name:     "Amadeus departure time (no timezone)",
			input:    `"2025-09-14T15:30:00"`,
			expected: time.Date(2025, 9, 14, 15, 30, 0, 0, time.UTC),
		},
	}

	type FlightEndPoint struct {
		IataCode string       `json:"iataCode"`
		At       FlexibleTime `json:"at"`
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData := fmt.Sprintf(`{"iataCode": "JFK", "at": %s}`, tt.input)

			var endpoint FlightEndPoint
			err := json.Unmarshal([]byte(jsonData), &endpoint)
			require.NoError(t, err)

			assert.Equal(t, "JFK", endpoint.IataCode)
			assert.True(t, tt.expected.Equal(endpoint.At.Time()))
			assert.False(t, endpoint.At.IsZero())
		})
	}
}
