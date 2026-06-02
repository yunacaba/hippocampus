// Command sample is a runnable demonstration of hippocampus: a typed agent that
// calls a tool and returns a typed result, wired to the OpenAI provider with
// end-user attribution.
//
// Run it with an OpenAI key:
//
//	OPENAI_API_KEY=sk-... go run ./sample
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/openai"
)

// --- Typed agent input and output -----------------------------------------

type RecommendationRequest struct {
	// FavoriteShows is the list of shows the user already likes.
	FavoriteShows []string `json:"favorite_shows"`
}

type RecommendationResponse struct {
	// ShowDescriptions are recommendations as "Title (Genre, Year)".
	ShowDescriptions []string `json:"show_descriptions"`
	// Explanation describes why these shows were chosen.
	Explanation string `json:"explanation"`
}

// --- A typed tool over a fake catalog --------------------------------------

type CatalogQuery struct {
	// Genre to search for (e.g. "comedy", "science fiction").
	Genre string `json:"genre"`
}

type CatalogResult struct {
	Shows []string `json:"shows"`
}

// fakeCatalog stands in for a real search backend.
var fakeCatalog = map[string][]string{
	"comedy":          {"Youthful (Comedy, 2024)", "Office Hours (Comedy, 2025)"},
	"science fiction": {"Star Trek: Lower Decks (Science Fiction, 2024)", "Orbital (Science Fiction, 2025)"},
	"drama":           {"The Long Road (Drama, 2024)", "Glasshouse (Drama, 2025)"},
}

func searchCatalog(_ context.Context, q *CatalogQuery, _ string) (*CatalogResult, error) {
	shows := fakeCatalog[q.Genre]
	if shows == nil {
		shows = []string{}
	}
	return &CatalogResult{Shows: shows}, nil
}

const promptTemplate = `You are a media recommendation agent.
Call the catalog_search tool (by genre) to find shows similar to the user's favorites.
Recommend the best 3 shows from the results. Only recommend shows from 2024 or 2025.
If you get no results, say "No results found".

Favorite shows:
{{range .favorite_shows}}- {{.}}
{{end}}`

func main() {
	if os.Getenv("OPENAI_API_KEY") == "" {
		fmt.Fprintln(os.Stderr, "Set OPENAI_API_KEY to run this sample, e.g.:")
		fmt.Fprintln(os.Stderr, "  OPENAI_API_KEY=sk-... go run ./sample")
		os.Exit(1)
	}

	// Wire the OpenAI provider with the env-var key provider.
	provider := openai.NewProvider(hippo.EnvKeyProvider{})

	agent, err := hippo.NewAgentWithTemplateText(
		promptTemplate,
		&RecommendationRequest{},
		&RecommendationResponse{
			ShowDescriptions: []string{"Title (Genre, Year)"},
			Explanation:      "Why these shows were chosen.",
		},
	).
		SetName("media_recommendation_agent").
		SetModel(provider, hippo.OpenAIGPT4OMini).
		AddTool(hippo.NewTool(
			"catalog_search",
			"Search the catalog for shows in a given genre.",
			searchCatalog,
			&CatalogQuery{},
			&CatalogResult{},
		)).
		SetMaxIterations(4).
		SetStructuredOutput(true). // ask OpenAI to return schema-conformant JSON
		Build()
	if err != nil {
		log.Fatalf("build agent: %v", err)
	}

	// Attribute every downstream LLM call to this end-user account ID.
	ctx := hippo.WithUserID(context.Background(), "sample-user-001")

	resp, err := agent.Execute(ctx, &RecommendationRequest{
		FavoriteShows: []string{"The Office", "Star Trek"},
	}, nil)
	if err != nil {
		log.Fatalf("execute: %v", err)
	}

	out, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println(string(out))
}
