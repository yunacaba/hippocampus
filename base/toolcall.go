package base

// FunctionCall is the parsed function-call payload surfaced by a model: a tool
// name and its JSON-encoded arguments.
type FunctionCall struct {
	// Name is the name of the function/tool to invoke.
	Name string `json:"name"`
	// Arguments is the JSON-encoded arguments object.
	Arguments string `json:"arguments"`
}

// ToolDelegateAction is the disposition of a tool call after a ToolDelegate has
// had a chance to rewrite it.
type ToolDelegateAction int

const (
	ToolDelegateActionPending ToolDelegateAction = iota
	ToolDelegateActionAccepted
	ToolDelegateActionRejected
	ToolDelegateActionModified
)

// ModelToolCall is the framework's view of a tool call surfaced by the model.
// It carries a delegate action so a ToolDelegate can accept, reject, or modify
// the call before execution.
type ModelToolCall struct {
	// ToolCallID is the unique identifier of the tool call.
	ToolCallID string `json:"id"`

	// FunctionCall is the function call to be executed.
	FunctionCall FunctionCall `json:"function,omitempty"`

	// DelegateAction is the disposition of the tool call.
	DelegateAction ToolDelegateAction `json:"delegate_action"`

	// RejectionReason is the reason the tool call was rejected (optional).
	RejectionReason string `json:"rejection_reason,omitempty"`
}

func (m *ModelToolCall) Accept() {
	m.DelegateAction = ToolDelegateActionAccepted
}

func (m *ModelToolCall) Reject() {
	m.RejectWithReason("no reason provided")
}

func (m *ModelToolCall) RejectWithReason(reason string) {
	m.DelegateAction = ToolDelegateActionRejected
	m.RejectionReason = reason
}

func (m *ModelToolCall) Modify(functionCall FunctionCall) {
	m.DelegateAction = ToolDelegateActionModified
	m.FunctionCall = functionCall
}
