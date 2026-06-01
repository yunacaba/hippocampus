package jsonx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yunacaba/hippocampus/internal/testdata"
)

func TestDeserializeFromPartialString(t *testing.T) {
	jsonString := `{"originalTitle":"A Tale of Two Cities II: A Third City Appears", "primaryTitle":"A Tale of Two Cities II: A Third City Appears", "genres":["Comedy","Drama","Docu`
	jsonValue := &testdata.TestQueryParams{}
	jsonValue, err := DeserializeFromPartialString(jsonString, jsonValue)
	require.NoError(t, err)
	assert.Equal(t, "A Tale of Two Cities II: A Third City Appears", jsonValue.OriginalTitle)
	assert.Equal(t, "A Tale of Two Cities II: A Third City Appears", jsonValue.PrimaryTitle)
	assert.Equal(t, []string{"Comedy", "Drama", "Docu"}, jsonValue.Genres)
	assert.Equal(t, 0, jsonValue.StartYearFrom)
	assert.Equal(t, 0, jsonValue.StartYearTo)
	assert.Equal(t, 0.0, jsonValue.AverageRatingFrom)
	assert.Equal(t, 0, len(jsonValue.SpokenLanguages))
}

func TestDeserializeAnyFromPartialString(t *testing.T) {
	jsonString := `{"originalTitle":"A Tale of Two Cities II: A Third City Appears", "primaryTitle":"A Tale of Two Cities II: A Third City Appears", "genres":["Comedy","Drama","Docu`
	jsonValue := &testdata.TestQueryParams{}
	err := DeserializeAnyFromPartialString(jsonString, jsonValue)
	require.NoError(t, err)
	assert.Equal(t, "A Tale of Two Cities II: A Third City Appears", jsonValue.OriginalTitle)
	assert.Equal(t, "A Tale of Two Cities II: A Third City Appears", jsonValue.PrimaryTitle)
	assert.Equal(t, []string{"Comedy", "Drama", "Docu"}, jsonValue.Genres)
}
