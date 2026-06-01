package hippocampus

import (
	"context"
	"fmt"
	"os"

	"github.com/yunacaba/hippocampus/base"
)

// KeyProvider supplies API keys to model providers, keyed by vendor. Carmen and
// other hosts can implement this over their own secret stores; EnvKeyProvider
// is the shipped default.
type KeyProvider interface {
	// APIKey returns the API key for the given vendor.
	APIKey(ctx context.Context, vendor base.LLMVendor) (string, error)
}

// EnvKeyProvider reads API keys from environment variables:
//
//	openai    -> OPENAI_API_KEY
//	anthropic -> ANTHROPIC_API_KEY
//	googleai  -> GOOGLE_AI_API_KEY
type EnvKeyProvider struct{}

var _ KeyProvider = EnvKeyProvider{}

func (EnvKeyProvider) APIKey(_ context.Context, vendor base.LLMVendor) (string, error) {
	var env string
	switch vendor.String() {
	case LLMVendorOpenAI.String():
		env = "OPENAI_API_KEY"
	case LLMVendorAnthropic.String():
		env = "ANTHROPIC_API_KEY"
	case LLMVendorGoogleAI.String():
		env = "GOOGLE_AI_API_KEY"
	default:
		return "", fmt.Errorf("hippocampus: unsupported vendor %q", vendor.String())
	}
	key := os.Getenv(env)
	if key == "" {
		return "", fmt.Errorf("hippocampus: %s is not set", env)
	}
	return key, nil
}
