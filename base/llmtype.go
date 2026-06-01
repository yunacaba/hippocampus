package base

// LLMVendor represents the LLM service vendor (ie. Anthropic, OpenAI, etc.)
type LLMVendor interface {
	IsValid() bool
	String() string
}

// LLMType represents the specific LLM model
type LLMType interface {
	Vendor() LLMVendor
	IsValid() bool
	String() string
}
