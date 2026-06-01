package hippocampus

const (

	// Gemini 3.0 Series (Latest - Preview)

	// Gemini 3 Pro - most advanced multimodal model
	// Outperformed major AI models in 19 out of 20 benchmarks on release
	// @Intelligence: 5
	// @Speed: 2
	// @Cost: TBD (pricing varies by region)
	// @Released: November 18, 2025 (preview)
	// @URL: https://ai.google.dev/gemini-api/docs/models
	GoogleAIGemini3Pro LLMType = "gemini-3-pro-preview"

	// Gemini 2.5 Series (Production)

	// Gemini 2.5 Pro - state-of-the-art thinking model with adaptive reasoning
	// Capable of reasoning through thoughts before responding, 2M token context
	// @Intelligence: 5
	// @Speed: 2
	// @Cost: TBD (pricing varies by region)
	// @Context: 2M tokens
	// @Released: June 17, 2025 (GA)
	// @URL: https://ai.google.dev/gemini-api/docs/models
	GoogleAIGemini25Pro LLMType = "gemini-2.5-pro"

	// Gemini 2.5 Flash - best price-performance with well-rounded capabilities
	// First stable 2.5 Flash model, optimized for production use
	// @Intelligence: 4
	// @Speed: 4
	// @Cost: TBD (pricing varies by region)
	// @Released: June 17, 2025 (GA)
	// @URL: https://ai.google.dev/gemini-api/docs/models
	GoogleAIGemini25Flash LLMType = "gemini-2.5-flash"

	// Gemini 2.5 Flash Lite - low-cost, high-performance model
	// Optimized for cost efficiency and low latency, supports high-throughput tasks
	// @Intelligence: 3
	// @Speed: 5
	// @Cost: TBD (pricing varies by region)
	// @Released: June 17, 2025
	// @URL: https://ai.google.dev/gemini-api/docs/models
	GoogleAIGemini25FlashLite LLMType = "gemini-2.5-flash-lite"

	// Gemini 2.0 Series

	// Gemini 2.0 Flash - highly efficient workhorse model with 1M token context
	// Low latency and enhanced performance, generally available
	// @Intelligence: 3
	// @Speed: 4
	// @Cost: TBD (pricing varies by region)
	// @Context: 1M tokens
	// @Released: January 30, 2025 (GA)
	// @URL: https://ai.google.dev/gemini-api/docs/models
	GoogleAIGemini20Flash LLMType = "gemini-2.0-flash"

	// Gemini 2.0 Flash Lite - most cost-efficient model
	// Optimized for cost efficiency and low latency
	// @Intelligence: 3
	// @Speed: 5
	// @Cost: TBD (pricing varies by region)
	// @Released: 2025
	// @URL: https://ai.google.dev/gemini-api/docs/models
	GoogleAIGemini20FlashLite LLMType = "gemini-2.0-flash-lite"
)
