package agenttest

import (
	"context"
	"testing"

	hippo "github.com/yunacaba/hippocampus"
	"github.com/yunacaba/hippocampus/base"
)

// The mock must not hand agent code a shape no adapter can produce.
//
// base.ModelCallResponse guarantees a non-nil GenerationInfo, and all three
// adapters honour it. Fixtures are written as `{Content: "…"}`, so without this
// the one remaining producer of the type consumed by real code diverges from
// production in exactly the dimension that was just made a guarantee.
func TestMockModelReturnsWritableGenerationInfo(t *testing.T) {
	model := NewMockModel(
		"generationinfo_test",
		hippo.AnthropicClaudeHaiku45,
		[]*base.ModelCallResponse{
			{Content: "hi"}, // the shape every fixture in this repo uses
		},
	)

	resp, err := model.Generate(context.Background(), base.ModelCallRequest{})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if resp.GenerationInfo == nil {
		t.Fatal("mock returned a nil GenerationInfo; no adapter can produce that")
	}
	resp.GenerationInfo["k"] = "v" // panics if nil
}
