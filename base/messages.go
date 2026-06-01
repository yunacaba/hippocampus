package base

import (
	"encoding/json"
	"fmt"
)

// Role identifies the speaker of a message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is a single turn in a conversation. A message has a role and one or
// more content parts. Mixed-modal messages (text + image + tool calls) are
// represented by multiple parts in a single message.
type Message struct {
	Role  Role
	Parts []ContentPart
}

// ContentPart is one piece of a message. It is a closed (sealed) sum type:
// the only implementations are TextPart, ImagePart, BinaryPart, ToolCallPart,
// and ToolResultPart, all defined in this package.
type ContentPart interface {
	isContentPart()
	// partType returns the discriminator used for JSON serialization.
	partType() string
}

// TextPart is plain text content.
type TextPart struct {
	Text string
}

func (TextPart) isContentPart()   {}
func (TextPart) partType() string { return "text" }

// ImagePart is an image referenced by URL. Supports both http(s):// and data: URLs.
type ImagePart struct {
	URL    string
	Detail string // "low", "high", "auto" (OpenAI); ignored by others.
}

func (ImagePart) isContentPart()   {}
func (ImagePart) partType() string { return "image" }

// BinaryPart is raw binary content with a MIME type (e.g. PDF for Anthropic).
type BinaryPart struct {
	MIMEType string
	Data     []byte
}

func (BinaryPart) isContentPart()   {}
func (BinaryPart) partType() string { return "binary" }

// ToolCallPart represents an assistant's request to invoke a tool.
type ToolCallPart struct {
	ToolCallID string
	Name       string
	Arguments  string // JSON-encoded arguments
}

func (ToolCallPart) isContentPart()   {}
func (ToolCallPart) partType() string { return "tool_call" }

// ToolResultPart represents the output of a tool invocation, sent back to the model.
type ToolResultPart struct {
	ToolCallID string
	Name       string
	Content    string
	IsError    bool
}

func (ToolResultPart) isContentPart()   {}
func (ToolResultPart) partType() string { return "tool_result" }

// partEnvelope is the wire form of a ContentPart: a flat object with a "type"
// discriminator and the union of all part fields. Empty fields are omitted.
type partEnvelope struct {
	Type       string `json:"type"`
	Text       string `json:"text,omitempty"`
	URL        string `json:"url,omitempty"`
	Detail     string `json:"detail,omitempty"`
	MIMEType   string `json:"mime_type,omitempty"`
	Data       []byte `json:"data,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Name       string `json:"name,omitempty"`
	Arguments  string `json:"arguments,omitempty"`
	Content    string `json:"content,omitempty"`
	IsError    bool   `json:"is_error,omitempty"`
}

// messageJSON is the wire form of a Message.
type messageJSON struct {
	Role  Role           `json:"role"`
	Parts []partEnvelope `json:"parts"`
}

// MarshalJSON implements json.Marshaler, encoding each content part with a
// "type" discriminator so the sealed sum type can be round-tripped.
func (m Message) MarshalJSON() ([]byte, error) {
	out := messageJSON{Role: m.Role, Parts: make([]partEnvelope, 0, len(m.Parts))}
	for _, p := range m.Parts {
		env := partEnvelope{Type: p.partType()}
		switch v := p.(type) {
		case TextPart:
			env.Text = v.Text
		case ImagePart:
			env.URL = v.URL
			env.Detail = v.Detail
		case BinaryPart:
			env.MIMEType = v.MIMEType
			env.Data = v.Data
		case ToolCallPart:
			env.ToolCallID = v.ToolCallID
			env.Name = v.Name
			env.Arguments = v.Arguments
		case ToolResultPart:
			env.ToolCallID = v.ToolCallID
			env.Name = v.Name
			env.Content = v.Content
			env.IsError = v.IsError
		default:
			return nil, fmt.Errorf("base: unknown content part type %T", p)
		}
		out.Parts = append(out.Parts, env)
	}
	return json.Marshal(out)
}

// UnmarshalJSON implements json.Unmarshaler, decoding each content part based on
// its "type" discriminator.
func (m *Message) UnmarshalJSON(data []byte) error {
	var in messageJSON
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	m.Role = in.Role
	m.Parts = make([]ContentPart, 0, len(in.Parts))
	for _, env := range in.Parts {
		var part ContentPart
		switch env.Type {
		case "text":
			part = TextPart{Text: env.Text}
		case "image":
			part = ImagePart{URL: env.URL, Detail: env.Detail}
		case "binary":
			part = BinaryPart{MIMEType: env.MIMEType, Data: env.Data}
		case "tool_call":
			part = ToolCallPart{ToolCallID: env.ToolCallID, Name: env.Name, Arguments: env.Arguments}
		case "tool_result":
			part = ToolResultPart{ToolCallID: env.ToolCallID, Name: env.Name, Content: env.Content, IsError: env.IsError}
		default:
			return fmt.Errorf("base: unknown content part type %q", env.Type)
		}
		m.Parts = append(m.Parts, part)
	}
	return nil
}
