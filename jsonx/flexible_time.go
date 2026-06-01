package jsonx

import (
	"encoding/json"
	"fmt"
	"time"
)

// FlexibleTime handles time fields that can be in different formats in API responses.
// This is common when external APIs return timestamps in various formats.
//
// Examples:
//   - API sometimes returns: "departure_time": "2025-09-14T15:30:00Z"
//   - API sometimes returns: "departure_time": "2025-09-14"
//   - API sometimes returns: "departure_time": "2025-09-14T15:30:00"
//
// All cases will be correctly parsed into a time.Time value.
// The priority order is: RFC3339 -> date-only (2006-01-02) -> datetime without timezone (2006-01-02T15:04:05).
type FlexibleTime struct {
	value time.Time
}

// UnmarshalJSON implements json.Unmarshaler for flexible time handling.
// It tries multiple time formats in priority order:
// 1. RFC3339 (standard format): "2025-09-14T15:30:00Z"
// 2. Date-only format: "2025-09-14"
// 3. Datetime without timezone: "2025-09-14T15:30:00"
func (f *FlexibleTime) UnmarshalJSON(data []byte) error {
	// First try unmarshaling as a native time.Time (RFC3339)
	if err := json.Unmarshal(data, &f.value); err == nil {
		return nil
	}

	// Fall back to string parsing with multiple formats
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return fmt.Errorf("field must be either an RFC3339 timestamp or a string representing a date/time")
	}

	// Try multiple time formats in order of preference
	formats := []string{
		time.RFC3339,          // "2025-09-14T15:30:00Z"
		"2006-01-02",          // "2025-09-14" - date only
		"2006-01-02T15:04:05", // "2025-09-14T15:30:00" - datetime without timezone
	}

	for _, format := range formats {
		if val, err := time.Parse(format, str); err == nil {
			f.value = val
			return nil
		}
	}

	return fmt.Errorf("cannot parse string %q as any supported time format (RFC3339, date-only, or datetime without timezone)", str)
}

// MarshalJSON implements json.Marshaler to serialize back to RFC3339 format.
func (f FlexibleTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.value)
}

// Time returns the underlying time.Time value.
func (f *FlexibleTime) Time() time.Time {
	return f.value
}

// String returns an RFC3339 string representation of the time.
func (f *FlexibleTime) String() string {
	return f.value.Format(time.RFC3339)
}

// IsZero returns true if the time value represents the zero time instant.
func (f *FlexibleTime) IsZero() bool {
	return f.value.IsZero()
}
