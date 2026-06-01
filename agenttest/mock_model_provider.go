package agenttest

import (
	"fmt"

	"github.com/yunacaba/hippocampus/base"
)

type MockModelProvider struct {
	base.ModelProvider
	Models map[string]base.Model
}

func NewMockModelProvider() *MockModelProvider {
	return &MockModelProvider{
		Models: make(map[string]base.Model),
	}
}

func (m *MockModelProvider) AddModel(name string, model base.Model) {
	m.Models[name] = model
}

func (m *MockModelProvider) Model(name string, llmType base.LLMType) (base.Model, error) {
	model, ok := m.Models[name]
	if !ok {
		return nil, fmt.Errorf("model not found: %s", name)
	}
	return model, nil
}
