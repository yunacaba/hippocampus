// Package agenttest provides mocks and helpers for testing code built on
// hippocampus. It is named agenttest rather than testing to avoid clashing with
// the standard library.
package agenttest

import (
	"context"
	"time"

	"github.com/yunacaba/hippocampus/base"
)

type MockModel struct {
	name                string
	llmType             base.LLMType
	responses           []*base.ModelCallResponse
	capturedInvocations []base.ModelCallRequest

	// Streaming simulation fields
	StreamingChunks []string      // Chunks to stream before final response
	StreamingDelay  time.Duration // Delay between chunks (for realistic timing)
}

var _ base.Model = &MockModel{}

func NewMockModel(
	name string,
	llmType base.LLMType,
	responses []*base.ModelCallResponse,
) *MockModel {
	return &MockModel{
		name:                name,
		llmType:             llmType,
		responses:           responses,
		capturedInvocations: []base.ModelCallRequest{},
		StreamingChunks:     nil,
		StreamingDelay:      0,
	}
}

// WithStreaming configures the mock model to simulate streaming responses.
func (m *MockModel) WithStreaming(chunks []string, delay time.Duration) *MockModel {
	m.StreamingChunks = chunks
	m.StreamingDelay = delay
	return m
}

func (m *MockModel) Name() string {
	return m.name
}

func (m *MockModel) LLMType() base.LLMType {
	return m.llmType
}

func (m *MockModel) LLMVendor() base.LLMVendor {
	return m.llmType.Vendor()
}

func (m *MockModel) Responses() []*base.ModelCallResponse {
	return m.responses
}

func (m *MockModel) CapturedInvocations() []base.ModelCallRequest {
	return m.capturedInvocations
}

func (m *MockModel) Generate(
	ctx context.Context,
	request base.ModelCallRequest,
) (*base.ModelCallResponse, error) {
	m.capturedInvocations = append(m.capturedInvocations, request)
	if len(m.responses) == 0 {
		return nil, nil
	}

	response := m.responses[len(m.capturedInvocations)-1]

	// Match what every real adapter guarantees: GenerationInfo is never nil
	// (see base.ModelCallResponse). Fixtures are written as `{Content: "…"}`
	// and so leave it nil, which would have tests exercise a response shape no
	// adapter can produce — and would turn every mock-based test red at once
	// the day generic post-processing starts adding a key, pointing the
	// diagnosis at fixtures rather than at anything real.
	response.GenerationInfo = base.WritableGenerationInfo(response.GenerationInfo)

	// If streaming is configured and a streaming function is provided, simulate streaming
	if request.StreamingFunc != nil && len(m.StreamingChunks) > 0 {
		for _, chunk := range m.StreamingChunks {
			// Add realistic timing delay between chunks if configured
			if m.StreamingDelay > 0 {
				time.Sleep(m.StreamingDelay)
			}

			// Call the streaming function with the chunk
			err := request.StreamingFunc(ctx, []byte(chunk))
			if err != nil {
				return nil, err
			}
		}
	}

	return response, nil
}
