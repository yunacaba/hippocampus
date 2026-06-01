package base

import "context"

type Model interface {
	Name() string
	LLMType() LLMType
	LLMVendor() LLMVendor
	Generate(
		ctx context.Context,
		request ModelCallRequest,
	) (*ModelCallResponse, error)
}
