package hippocampus

const (

	// GPT-5 Series - Latest generation models

	// GPT-5.1 - next model balancing intelligence and speed for agentic and coding tasks
	// Dynamically adapts thinking time based on task complexity
	// @Intelligence: 5
	// @Speed: 3
	// @Cost: TBD (pricing not yet public)
	// @Released: November 13, 2025
	// @URL: https://openai.com/index/gpt-5-1-for-developers/
	OpenAIGPT51 LLMType = "gpt-5.1-2025-11-13"

	// GPT-5 - full reasoning model with significant leap in intelligence
	// State-of-the-art performance across coding, math, writing, health, visual perception
	// @Intelligence: 5
	// @Speed: 2
	// @Cost: TBD (pricing not yet public)
	// @Released: August 7, 2025
	// @URL: https://openai.com/index/introducing-gpt-5/
	OpenAIGPT5 LLMType = "gpt-5-2025-08-07"

	// GPT-5 Mini - smaller, more cost-effective version of GPT-5
	// Optimized for high-volume use cases while maintaining quality
	// @Intelligence: 4
	// @Speed: 3
	// @Cost: TBD (pricing not yet public)
	// @Released: August 7, 2025
	// @URL: https://openai.com/index/introducing-gpt-5-for-developers/
	OpenAIGPT5Mini LLMType = "gpt-5-mini-2025-08-07"

	// GPT-5 Nano - most compact variant for high-throughput applications
	// Fast and efficient for simple to moderate complexity tasks
	// @Intelligence: 3
	// @Speed: 4
	// @Cost: TBD (pricing not yet public)
	// @Released: August 7, 2025
	// @URL: https://openai.com/index/introducing-gpt-5-for-developers/
	OpenAIGPT5Nano LLMType = "gpt-5-nano-2025-08-07"

	// GPT-4 Series

	// GPT-4.1 - excels at instruction following and tool calling
	// Features 1M token context window and low latency without reasoning step
	// @Intelligence: 4
	// @Speed: 3
	// @Cost: $2/$.5/$8 ICO/million
	// @Released: April 14, 2025
	// @URL: https://openai.com/index/gpt-4-1/
	OpenAIGPT41 LLMType = "gpt-4.1-2025-04-14"

	// GPT-4o - versatile, high-intelligence flagship model
	// Accepts text and image inputs, produces text outputs
	// @Intelligence: 3
	// @Speed: 3
	// @Cost: $2.5/$10 IO/million
	// @URL: https://platform.openai.com/docs/models/gpt-4o
	OpenAIGPT4O LLMType = "gpt-4o"

	// GPT-4o Mini - fast, affordable model for focused tasks
	// Accepts text and image inputs, produces text outputs
	// @Intelligence: 2
	// @Speed: 4
	// @Cost: $0.15/$0.6 IO/million
	// @URL: https://platform.openai.com/docs/models/gpt-4o-mini
	OpenAIGPT4OMini LLMType = "gpt-4o-mini"
)
