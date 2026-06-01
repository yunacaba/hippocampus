package base

type ModelProvider interface {
	Model(name string, modelType LLMType) (Model, error)
}
