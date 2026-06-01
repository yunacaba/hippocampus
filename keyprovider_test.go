package hippocampus_test

import (
	"context"
	"testing"

	hippo "github.com/yunacaba/hippocampus"
)

func TestEnvKeyProvider_NilVendor(t *testing.T) {
	// LLMType.Vendor() returns nil for an unknown model; APIKey must return an
	// error, not panic on a nil-interface method call.
	nilVendor := hippo.LLMType("bogus-model").Vendor()
	if nilVendor != nil {
		t.Fatalf("expected nil vendor for unknown model, got %v", nilVendor)
	}

	_, err := hippo.EnvKeyProvider{}.APIKey(context.Background(), nilVendor)
	if err == nil {
		t.Fatal("expected an error for a nil vendor, got nil")
	}
}

func TestEnvKeyProvider_KnownVendor(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test-123")
	key, err := hippo.EnvKeyProvider{}.APIKey(context.Background(), hippo.OpenAIGPT4O.Vendor())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "sk-test-123" {
		t.Errorf("key = %q, want sk-test-123", key)
	}
}

func TestEnvKeyProvider_MissingKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	if _, err := (hippo.EnvKeyProvider{}).APIKey(context.Background(), hippo.AnthropicClaudeHaiku45.Vendor()); err == nil {
		t.Error("expected an error when the env var is unset")
	}
}
