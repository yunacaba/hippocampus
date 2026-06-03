package hippocampus

import (
	"github.com/yunacaba/hippocampus/base"
)

// LLMVendor represents the LLM service vendor (ie. Anthropic, OpenAI, etc.)
type LLMVendor string

const (
	LLMVendorAnthropic LLMVendor = "anthropic"
	LLMVendorOpenAI    LLMVendor = "openai"
	LLMVendorGoogleAI  LLMVendor = "googleai"
	// LLMVendorOllama labels models served by a local Ollama runtime via the
	// langchain adapter's native (/api/chat) backend. Ollama model names are
	// arbitrary and have no predefined LLMType constant, so Vendor() never
	// derives this from a name; it is set by the Ollama provider itself.
	LLMVendorOllama LLMVendor = "ollama"
)

// LLMType represents the specific LLM model.
//
// Model constants for every vendor and the Vendor() dispatch live together in
// this package (see llmtypes_*.go) rather than in the provider adapter
// subpackages: the agent loop casts base.LLMType back to this concrete type for
// SupportsCustomTemperature(), so the constants must be reachable here without
// importing the adapters (which would form an import cycle).
type LLMType string

var (
	_ base.LLMType   = LLMType("")
	_ base.LLMVendor = LLMVendor("")
)

// Vendor returns the LLMVendor for a given LLMType.
func (m LLMType) Vendor() base.LLMVendor {
	switch m {
	case AnthropicClaudeOpus45,
		AnthropicClaudeSonnet45,
		AnthropicClaudeHaiku45:
		return LLMVendorAnthropic
	case OpenAIGPT51,
		OpenAIGPT5,
		OpenAIGPT5Mini,
		OpenAIGPT5Nano,
		OpenAIGPT41,
		OpenAIGPT4O,
		OpenAIGPT4OMini:
		return LLMVendorOpenAI
	case GoogleAIGemini3Pro,
		GoogleAIGemini25Pro,
		GoogleAIGemini25Flash,
		GoogleAIGemini25FlashLite,
		GoogleAIGemini20Flash,
		GoogleAIGemini20FlashLite:
		return LLMVendorGoogleAI
	default:
		return nil
	}
}

// IsValid returns true if the LLMType is a known model.
func (m LLMType) IsValid() bool {
	return m.Vendor() != nil
}

// String returns the string representation of the LLMType.
func (m LLMType) String() string {
	return string(m)
}

// SupportsCustomTemperature returns true if the model supports custom
// temperature values (e.g. temperature=0). Some models only support the default
// temperature and error with custom values.
func (m LLMType) SupportsCustomTemperature() bool {
	switch m {
	case OpenAIGPT51,
		OpenAIGPT5,
		OpenAIGPT5Mini,
		OpenAIGPT5Nano,
		OpenAIGPT41:
		// These models only support default temperature=1.0
		return false
	default:
		// Most models support temperature=0 for consistent results
		return true
	}
}

// String returns the string representation of the vendor.
func (v LLMVendor) String() string {
	return string(v)
}

// IsValid returns true if the vendor is recognized.
func (v LLMVendor) IsValid() bool {
	switch v {
	case LLMVendorAnthropic, LLMVendorOpenAI, LLMVendorGoogleAI, LLMVendorOllama:
		return true
	default:
		return false
	}
}
