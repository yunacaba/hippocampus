package hippocampus

type agentArgs[I any, O any] struct {
	templateText      string               // For backward compatibility
	promptTemplate    PromptTemplate[I, O] // For new interface-based approach
	sampleArg         I
	sampleResponse    O
	name              string
	modelProvider     ModelProvider
	observer          AgentObserver[O]
	delegate          ToolDelegate
	tracer            Tracer
	llmType           LLMType
	tools             []AnyTool
	debugToolCalls    bool
	maxIterations     int
	toolCallingPolicy ToolCallingPolicy
}

type NameAgentBuilder[I any, O any] struct {
	args agentArgs[I, O]
}

func NewAgent[I any, O any](
	promptTemplate PromptTemplate[I, O],
	sampleArg I,
	sampleResponse O,
) *NameAgentBuilder[I, O] {
	return &NameAgentBuilder[I, O]{
		args: agentArgs[I, O]{
			promptTemplate:    promptTemplate,
			sampleArg:         sampleArg,
			sampleResponse:    sampleResponse,
			tools:             []AnyTool{},
			debugToolCalls:    false,
			maxIterations:     5,
			toolCallingPolicy: ToolCallingAnyIteration, // Default: allow tools in any iteration
		},
	}
}

// NewAgentWithTemplateText creates an agent using the template string approach.
func NewAgentWithTemplateText[I any, O any](
	templateText string,
	sampleArg I,
	sampleResponse O,
) *NameAgentBuilder[I, O] {
	return &NameAgentBuilder[I, O]{
		args: agentArgs[I, O]{
			sampleArg:         sampleArg,
			sampleResponse:    sampleResponse,
			templateText:      templateText,
			tools:             []AnyTool{},
			debugToolCalls:    false,
			maxIterations:     5,
			toolCallingPolicy: ToolCallingAnyIteration, // Default: allow tools in any iteration
		},
	}
}

func (b *NameAgentBuilder[I, O]) SetName(name string) *ModelAgentBuilder[I, O] {
	b.args.name = name
	return &ModelAgentBuilder[I, O]{args: b.args}
}

type ModelAgentBuilder[I any, O any] struct {
	args agentArgs[I, O]
}

func (b *ModelAgentBuilder[I, O]) SetModel(
	modelProvider ModelProvider,
	llmType LLMType,
) *OptionsAgentBuilder[I, O] {
	b.args.modelProvider = modelProvider
	b.args.llmType = llmType
	return &OptionsAgentBuilder[I, O]{args: b.args}
}

type OptionsAgentBuilder[I any, O any] struct {
	args agentArgs[I, O]
}

func (b *OptionsAgentBuilder[I, O]) SetObserver(observer AgentObserver[O]) *OptionsAgentBuilder[I, O] {
	b.args.observer = observer
	return b
}

func (b *OptionsAgentBuilder[I, O]) SetToolDelegate(delegate ToolDelegate) *OptionsAgentBuilder[I, O] {
	b.args.delegate = delegate
	return b
}

// SetTracer sets the tracer used for agent, tool, and model spans. The default
// is NoopTracer.
func (b *OptionsAgentBuilder[I, O]) SetTracer(tracer Tracer) *OptionsAgentBuilder[I, O] {
	b.args.tracer = tracer
	return b
}

func (b *OptionsAgentBuilder[I, O]) SetDebugToolCalls(debugToolCalls bool) *OptionsAgentBuilder[I, O] {
	b.args.debugToolCalls = debugToolCalls
	return b
}

func (b *OptionsAgentBuilder[I, O]) SetMaxIterations(maxIterations int) *OptionsAgentBuilder[I, O] {
	b.args.maxIterations = maxIterations
	return b
}

func (b *OptionsAgentBuilder[I, O]) SetToolCallingPolicy(policy ToolCallingPolicy) *OptionsAgentBuilder[I, O] {
	b.args.toolCallingPolicy = policy
	return b
}

func (b *OptionsAgentBuilder[I, O]) AddTool(tool AnyTool) *OptionsAgentBuilder[I, O] {
	b.args.tools = append(b.args.tools, tool)
	return b
}

func (b *OptionsAgentBuilder[I, O]) AddTools(tools []AnyTool) *OptionsAgentBuilder[I, O] {
	b.args.tools = append(b.args.tools, tools...)
	return b
}

func (b *OptionsAgentBuilder[I, O]) Build() (*Agent[I, O], error) {
	model, err := b.args.modelProvider.Model(b.args.name, b.args.llmType)
	if err != nil {
		return nil, err
	}

	var agentInstance *Agent[I, O]

	// Choose creation method based on what was provided
	if b.args.promptTemplate != nil {
		agentInstance, err = newAgent(
			b.args.name,
			model,
			b.args.observer,
			b.args.delegate,
			b.args.promptTemplate,
			b.args.tools,
			b.args.tracer,
		)
	} else {
		agentInstance, err = newAgentWithTextTemplate(
			b.args.name,
			model,
			b.args.observer,
			b.args.delegate,
			b.args.templateText,
			b.args.sampleArg,
			b.args.sampleResponse,
			b.args.tools,
			b.args.tracer,
		)
	}

	if err != nil {
		return nil, err
	}

	agentInstance.debugToolCalls = b.args.debugToolCalls
	agentInstance.maxIterations = b.args.maxIterations
	agentInstance.toolCallingPolicy = b.args.toolCallingPolicy
	return agentInstance, nil
}
