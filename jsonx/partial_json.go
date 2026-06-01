package jsonx

import (
	p "github.com/blaze2305/partial-json-parser"
	o "github.com/blaze2305/partial-json-parser/options"
)

// Parses a partial string that might result from streaming.
// Don't use types that have required fields.
//
// Deprecated: prefer DeserializeLLM, which also strips markdown fences and
// surrounding prose, repairs a wider range of malformed JSON, and reports
// whether repair was needed.
func DeserializeFromPartialString[T any](jsonString string, prototype T) (T, error) {
	fixedJSONString, err := p.ParseMalformedString(jsonString, o.ALL, true)
	if err != nil {
		return prototype, err
	}
	return Deserialize(fixedJSONString, prototype)
}

// Parses a partial string that might result from streaming.
// Don't use types that have required fields.
//
// Deprecated: prefer DeserializeAnyLLM.
func DeserializeAnyFromPartialString(jsonString string, result any) error {
	fixedJSONString, err := p.ParseMalformedString(jsonString, o.ALL, true)
	if err != nil {
		return err
	}
	return DeserializeAny(fixedJSONString, result)
}
