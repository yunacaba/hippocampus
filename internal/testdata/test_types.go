// Package testdata holds shared struct fixtures used across hippocampus tests.
package testdata

import "time"

// Test structs

// agent:arg tool:arg
type Address struct {
	// Street address. Woo-hoo!
	Street string `required:"true"`
	// City name. Woo-hoo!
	City string `required:"true"`
	// Postal/ZIP code. Woo-hoo!
	PostalCode string `required:"true"`
}

type TripStatus string

const (
	TripStatusUnknown    TripStatus = "UNKNOWN"
	TripStatusPlanned    TripStatus = "PLANNED"
	TripStatusInProgress TripStatus = "IN_PROGRESS"
	TripStatusCompleted  TripStatus = "COMPLETED"
	TripStatusCancelled  TripStatus = "CANCELLED"
)

// agent:arg tool:arg
type TravelPreferences struct {
	// Maximum budget for the trip
	MaxBudget float64 `json:"max_budget" min:"0"`
	// Trip start date
	StartDate time.Time `json:"start_date"`
	// Trip duration in days
	Duration int `json:"duration" min:"1" max:"90"`
	// List of destinations
	Destinations []string `json:"destinations"`
	// Traveler's home address
	HomeAddress Address `json:"home_address"`
	// Travel preferences flags
	Preferences map[string]bool `json:"preferences"`
	// Current status of the trip
	Status TripStatus `json:"status"`
}

type TestToolArg struct {
	Name           string    `json:"name"`
	Address        Address   `json:"address" required:"true"`
	Age            int       `json:"age" required:"true"`
	FavoriteColors []string  `json:"favorite_colors" required:"true"`
	IsAdmin        bool      `json:"is_admin" required:"true"`
	StartDate      time.Time `json:"start_date" required:"true"`
}

// tool:result
type TestToolResult struct {
	Status              string `json:"status"`
	Error               string `json:"error,omitempty"`
	SomeInterestingCode int    `json:"some_interesting_code"`
}

type TestArrayFormatting struct {
	Addresses []Address `json:"addresses"`
	Names     []string  `json:"names"`
	Numbers   []int     `json:"numbers"`
	Floats    []float64 `json:"floats"`
	Booleans  []bool    `json:"booleans"`
}

type TestQueryParams struct {
	// Title in original language.
	OriginalTitle string `json:"originalTitle,omitempty"`
	// Title in English.
	PrimaryTitle string `json:"primaryTitle"`
	// Genres, must be the following: Comedy, Drama, Documentary, Action, Romance, Thriller, Crime, Horror, Adventure, Fantasy, Sci-Fi
	Genres []string `json:"genres,omitempty"`
	// Earliest year of release.
	StartYearFrom int `json:"startYearFrom,omitempty"`
	// Latest year of release.
	StartYearTo int `json:"startYearTo,omitempty"`
	// Lowest average rating to return.
	AverageRatingFrom float64 `json:"averageRatingFrom,omitempty"`
	// Languages in ISO 639-1 format.
	SpokenLanguages []string `json:"spokenLanguages,omitempty"`
}

// tool:result
type TestImdbTitle struct {
	PrimaryTitle  string   `json:"primaryTitle"`
	OriginalTitle string   `json:"originalTitle"`
	StartYear     int      `json:"startYear"`
	Genres        []string `json:"genres"`
}

// tool:result
type TestQueryResult struct {
	Results []TestImdbTitle `json:"results"`
}
