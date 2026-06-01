package hippocampus

import "context"

// Attribute is a provider-neutral key/value annotation attached to a span. It
// replaces OpenTelemetry's attribute.KeyValue in the framework's public surface
// so importing hippocampus does not pull in OpenTelemetry. Value should be a
// string, bool, int64, or float64; tracer implementations decide how to encode
// other types.
type Attribute struct {
	Key   string
	Value any
}

// StringAttr builds a string-valued Attribute.
func StringAttr(key, value string) Attribute { return Attribute{Key: key, Value: value} }

// IntAttr builds an int-valued Attribute (stored as int64).
func IntAttr(key string, value int) Attribute { return Attribute{Key: key, Value: int64(value)} }

// Int64Attr builds an int64-valued Attribute.
func Int64Attr(key string, value int64) Attribute { return Attribute{Key: key, Value: value} }

// BoolAttr builds a bool-valued Attribute.
func BoolAttr(key string, value bool) Attribute { return Attribute{Key: key, Value: value} }

// Span is a single unit of traced work. It is a minimal, provider-neutral
// subset of what OpenTelemetry offers.
type Span interface {
	// End completes the span.
	End()
	// SetAttributes annotates the span with key/value attributes.
	SetAttributes(attrs ...Attribute)
	// AddEvent records a named event on the span, with optional attributes.
	AddEvent(name string, attrs ...Attribute)
	// RecordError records an error on the span.
	RecordError(err error)
}

// Tracer starts spans. The default is NoopTracer; provide an OpenTelemetry-backed
// implementation via the agent builder's SetTracer to emit real traces.
type Tracer interface {
	// StartSpan starts a new span and returns a context carrying it.
	StartSpan(ctx context.Context, name string) (context.Context, Span)
}

// NoopTracer is a Tracer that does nothing. It is the default when no tracer is
// configured.
type NoopTracer struct{}

var _ Tracer = NoopTracer{}

func (NoopTracer) StartSpan(ctx context.Context, _ string) (context.Context, Span) {
	return ctx, noopSpan{}
}

type noopSpan struct{}

var _ Span = noopSpan{}

func (noopSpan) End()                          {}
func (noopSpan) SetAttributes(...Attribute)    {}
func (noopSpan) AddEvent(string, ...Attribute) {}
func (noopSpan) RecordError(error)             {}
