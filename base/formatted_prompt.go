package base

// FormattedPrompt is a prompt with its sample response and response schema.
type FormattedPrompt struct {
	Prompt         string
	SampleResponse string
	ResponseSchema string
}
