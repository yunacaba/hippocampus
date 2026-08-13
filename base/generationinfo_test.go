package base_test

import (
	"testing"

	"github.com/yunacaba/hippocampus/base"
)

func TestWritableGenerationInfoIsAlwaysWritable(t *testing.T) {
	// The property, exercised the way the failure happens: by writing. A nil map
	// reads fine and passes any length or equality check, so a test that only
	// inspected the result would pass against the very bug this prevents.
	got := base.WritableGenerationInfo(nil)
	if got == nil {
		t.Fatal("nil in, nil out")
	}
	got["k"] = "v" // panics if this returned nil

	if len(got) != 1 {
		t.Errorf("len = %d, want 1", len(got))
	}
}

func TestWritableGenerationInfoKeepsWhatItWasGiven(t *testing.T) {
	// Not a copy: adapters pass provider data through this, and the caller
	// expects the values, not a sanitised empty map.
	in := map[string]any{"InputTokens": 7}
	got := base.WritableGenerationInfo(in)

	if got["InputTokens"] != 7 {
		t.Errorf("InputTokens = %v, want 7", got["InputTokens"])
	}

	// And the same map, so a write lands where the caller can see it.
	got["extra"] = true
	if in["extra"] != true {
		t.Error("a non-nil map should be returned as-is, not copied")
	}
}
