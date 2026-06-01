package hippocampus_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/internal/testdata"
)

// Note: carmen's "template with proto arg" subtest is intentionally not ported
// here. It relied on a carmen proto type whose exported Title field serialized
// via encoding/json after SerializeToMap flattening. The meaningful protojson
// path (jsonx.Deserialize/SerializeToString on a proto.Message) is covered by
// TestTool_ProtocolBufferHandling instead.

func TestPromptTemplate(t *testing.T) {
	sampleResponse := testdata.TravelPreferences{
		MaxBudget:    5000.0,
		StartDate:    time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		Duration:     14,
		Destinations: []string{"Paris", "Rome", "Barcelona"},
		HomeAddress: testdata.Address{
			Street:     "123 Main St",
			City:       "Springfield",
			PostalCode: "12345",
		},
		Preferences: map[string]bool{
			"needs_visa":   true,
			"has_passport": true,
			"window_seat":  true,
			"vegetarian":   false,
		},
	}

	t.Run("successful template creation and formatting", func(t *testing.T) {
		templateText := `Plan a trip with the following requirements:
Budget: {{.max_budget}}
Start Date: {{.start_date}}
Duration: {{.duration}} days
Destinations: {{range .destinations}}
- {{.}}{{end}}

Home Address:
{{.home_address.Street}}
{{.home_address.City}}
{{.home_address.PostalCode}}

Preferences:{{range $key, $value := .preferences}}
{{$key}}: {{$value}}{{end}}`

		textTemplate, err := hippo.NewTextPromptTemplate(
			templateText,
			&sampleResponse,
			&sampleResponse,
		)
		require.NoError(t, err)
		require.NotNil(t, textTemplate)

		fields := textTemplate.GetFields()
		assert.NotEmpty(t, fields)

		var foundBudget, foundStreet bool
		for _, field := range fields {
			switch field.Name {
			case "max_budget":
				foundBudget = true
			case "home_address.Street":
				foundStreet = true
			}
		}
		assert.True(t, foundBudget, "max_budget field not found")
		assert.True(t, foundStreet, "home_address.Street field not found")

		data := testdata.TravelPreferences{
			MaxBudget:    5000.0,
			StartDate:    time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			Duration:     14,
			Destinations: []string{"Paris", "Rome", "Barcelona"},
			HomeAddress: testdata.Address{
				Street:     "123 Main St",
				City:       "Springfield",
				PostalCode: "12345",
			},
			Preferences: map[string]bool{
				"needs_visa":   true,
				"has_passport": true,
				"window_seat":  true,
				"vegetarian":   false,
			},
			Status: testdata.TripStatusPlanned,
		}

		result, err := textTemplate.GeneratePrompt(context.Background(), &data)
		require.NoError(t, err)
		assert.Contains(t, result.Prompt, "Budget: 5000")
		assert.Contains(t, result.Prompt, "Duration: 14 days")
		assert.Contains(t, result.Prompt, "123 Main St")
		assert.Contains(t, result.Prompt, "Springfield")
		assert.Contains(t, result.Prompt, "- Paris")
		assert.Contains(t, result.Prompt, "- Rome")
		assert.Contains(t, result.Prompt, "- Barcelona")
		assert.Contains(t, result.Prompt, "window_seat: true")
	})

	t.Run("template with array access", func(t *testing.T) {
		templateText := `First destination: {{index .destinations 0}}`

		tmpl, err := hippo.NewTextPromptTemplate(
			templateText,
			&sampleResponse,
			&sampleResponse,
		)
		require.NoError(t, err)

		data := testdata.TravelPreferences{
			Destinations: []string{"Tokyo"},
			Preferences:  map[string]bool{"needs_visa": true},
			Status:       testdata.TripStatusPlanned,
		}

		result, err := tmpl.GeneratePrompt(context.Background(), &data)
		require.NoError(t, err)
		assert.Contains(t, result.Prompt, "First destination: Tokyo")
	})

	t.Run("template with map values", func(t *testing.T) {
		templateText := `Needs visa: {{index .preferences "needs_visa"}}`

		tmpl, err := hippo.NewTextPromptTemplate(
			templateText,
			&sampleResponse,
			&sampleResponse,
		)
		require.NoError(t, err)

		data := testdata.TravelPreferences{
			Destinations: []string{"Tokyo"},
			Preferences:  map[string]bool{"needs_visa": true},
			Status:       testdata.TripStatusPlanned,
		}

		result, err := tmpl.GeneratePrompt(context.Background(), &data)
		require.NoError(t, err)
		assert.Contains(t, result.Prompt, "Needs visa: true")
	})

	t.Run("template with enum values", func(t *testing.T) {
		templateText := `Trip status: {{.status}}`

		tmpl, err := hippo.NewTextPromptTemplate(
			templateText,
			&sampleResponse,
			&sampleResponse,
		)
		require.NoError(t, err)

		data := testdata.TravelPreferences{
			Destinations: []string{"Tokyo"},
			Preferences:  map[string]bool{"needs_visa": true},
			Status:       testdata.TripStatusInProgress,
		}

		result, err := tmpl.GeneratePrompt(context.Background(), &data)
		require.NoError(t, err)
		assert.Contains(t, result.Prompt, "Trip status: IN_PROGRESS")

		data.Status = testdata.TripStatusCompleted
		result, err = tmpl.GeneratePrompt(context.Background(), &data)
		require.NoError(t, err)
		assert.Contains(t, result.Prompt, "Trip status: COMPLETED")
	})

	t.Run("template with array formatting", func(t *testing.T) {
		templateText := `addresses:{{range .addresses}}
- {{.Street}} {{.City}} {{.PostalCode}}{{end}}

names:{{range .names}}
- {{.}}{{end}}

numbers:{{range .numbers}}
- {{.}}{{end}}

floats:{{range .floats}}
- {{.}}{{end}}

booleans:{{range .booleans}}
- {{.}}{{end}}`

		arrayFormatting := testdata.TestArrayFormatting{
			Names:     []string{"John Doe", "Jane Whittaker"},
			Numbers:   []int{1, 2},
			Floats:    []float64{1.1, 2.2},
			Booleans:  []bool{true, false},
			Addresses: []testdata.Address{{Street: "123 Main St", City: "Springfield", PostalCode: "12345"}},
		}

		tmpl, err := hippo.NewTextPromptTemplate(
			templateText,
			&arrayFormatting,
			&arrayFormatting,
		)
		require.NoError(t, err)

		result, err := tmpl.GeneratePrompt(context.Background(), &arrayFormatting)
		require.NoError(t, err)
		assert.Equal(t, `addresses:
- 123 Main St Springfield 12345

names:
- John Doe
- Jane Whittaker

numbers:
- 1
- 2

floats:
- 1.1
- 2.2

booleans:
- true
- false`, result.Prompt)
	})

	t.Run("template with missing non-required key", func(t *testing.T) {
		templateText := `This is a test: {{.non_existent_field}}`

		tmpl, err := hippo.NewTextPromptTemplate(
			templateText,
			&sampleResponse,
			&sampleResponse,
		)
		require.NoError(t, err)

		_, err = tmpl.GeneratePrompt(context.Background(), &sampleResponse)
		assert.Error(t, err, "Format should fail due to missing key")
		assert.Contains(t, err.Error(), `map has no entry for key "non_existent_field"`, "Error should mention the missing key")
	})
}
