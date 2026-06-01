package jsonx

import (
	sysjson "encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/kaptinlin/jsonrepair"
)

// This file provides tolerant parsing of JSON emitted by LLMs, which routinely
// wrap output in markdown fences, surround it with prose, or truncate it when a
// token limit is hit. CleanLLMJSON normalizes the text; DeserializeLLM and
// DeserializeAnyLLM clean, then unmarshal, then attempt repair on failure.

// CleanLLMJSON is the full cleaning pipeline for an LLM response: it strips any
// markdown code fences and then extracts the first complete JSON object from
// surrounding prose. It does not repair malformed JSON — see DeserializeLLM.
func CleanLLMJSON(response string) string {
	return ExtractJSONObject(CleanMarkdownBlock(response))
}

// CleanMarkdownBlock removes markdown code-block wrappers (```json ... ```) from
// a response. It handles complete blocks, blocks that appear mid-response, and
// truncated responses where the model hit a token limit before the closing fence.
func CleanMarkdownBlock(response string) string {
	cleaned := strings.TrimSpace(response)

	// Code block at the start.
	if strings.HasPrefix(cleaned, "```") {
		startIdx := strings.Index(cleaned, "\n")
		if startIdx == -1 {
			return cleaned
		}
		endIdx := strings.LastIndex(cleaned, "```")
		if endIdx > startIdx {
			return strings.TrimSpace(cleaned[startIdx+1 : endIdx])
		}
		// Unclosed (truncated) block — strip the opening fence line.
		return strings.TrimSpace(cleaned[startIdx+1:])
	}

	// Code block somewhere in the middle.
	fenceIdx := strings.Index(cleaned, "\n```")
	if fenceIdx == -1 {
		return cleaned
	}
	fenceStart := fenceIdx + 1 // skip the \n before ```
	contentStart := strings.Index(cleaned[fenceStart:], "\n")
	if contentStart == -1 {
		return cleaned
	}
	contentStart += fenceStart + 1 // absolute index past the fence line's \n
	endIdx := strings.LastIndex(cleaned, "```")
	if endIdx > contentStart {
		return strings.TrimSpace(cleaned[contentStart:endIdx])
	}
	return strings.TrimSpace(cleaned[contentStart:])
}

// ExtractJSONObject finds and extracts the first complete JSON object from text,
// handling explanatory prose before or after it. Brace matching tracks string
// context so braces inside string values are ignored.
func ExtractJSONObject(response string) string {
	cleaned := strings.TrimSpace(response)

	startIdx := strings.Index(cleaned, "{")
	if startIdx == -1 {
		return cleaned
	}

	depth := 0
	endIdx := -1
	inString := false
	prevChar := byte(0)

	for i := startIdx; i < len(cleaned); i++ {
		c := cleaned[i]

		if c == '"' && prevChar != '\\' {
			inString = !inString
		} else if !inString {
			switch c {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					endIdx = i
				}
			}
		}

		if endIdx != -1 {
			break
		}
		prevChar = c
	}

	if endIdx != -1 {
		return strings.TrimSpace(cleaned[startIdx : endIdx+1])
	}
	return cleaned
}

// DeserializeLLM cleans an LLM response and deserializes it into a fresh value of
// type T (using prototype to determine the target type, like Deserialize). On a
// parse failure it attempts JSON repair, then a truncation-close fallback. The
// returned bool reports whether repair was needed; callers should treat repaired
// results as potentially partial.
func DeserializeLLM[T any](response string, prototype T) (T, bool, error) {
	cleaned := CleanLLMJSON(response)

	value, err := Deserialize[T](cleaned, prototype)
	if err == nil {
		return value, false, nil
	}
	firstErr := err

	// Repair common issues: trailing commas, missing quotes, light truncation.
	if fixed, rerr := jsonrepair.Repair(cleaned); rerr == nil && fixed != cleaned {
		if v, e := Deserialize[T](fixed, prototype); e == nil {
			return v, true, nil
		}
	}

	// Fallback: close containers left open by heavy truncation.
	if closed, ok := closeTruncatedJSON(cleaned); ok {
		if v, e := Deserialize[T](closed, prototype); e == nil {
			return v, true, nil
		}
	}

	var zero T
	return zero, false, fmt.Errorf("failed to parse JSON: %w\n%s", firstErr, errorContext(cleaned, firstErr))
}

// DeserializeAnyLLM cleans an LLM response and deserializes it in place into the
// pointer v, with the same repair fallbacks as DeserializeLLM. Returns whether
// repair was needed.
func DeserializeAnyLLM(response string, v any) (bool, error) {
	cleaned := CleanLLMJSON(response)

	err := DeserializeAny(cleaned, v)
	if err == nil {
		return false, nil
	}
	firstErr := err

	if fixed, rerr := jsonrepair.Repair(cleaned); rerr == nil && fixed != cleaned {
		if e := DeserializeAny(fixed, v); e == nil {
			return true, nil
		}
	}

	if closed, ok := closeTruncatedJSON(cleaned); ok {
		if e := DeserializeAny(closed, v); e == nil {
			return true, nil
		}
	}

	return false, fmt.Errorf("failed to parse JSON: %w\n%s", firstErr, errorContext(cleaned, firstErr))
}

// SalvageArrayElements locates the named top-level array in text and returns each
// well-formed element as raw JSON, skipping any element that can't be parsed.
// Useful as a last-resort recovery when a single malformed element would
// otherwise discard an entire payload. Returns nil if the field is absent or not
// an array. Element boundaries respect string context.
func SalvageArrayElements(text, fieldName string) [][]byte {
	needle := `"` + fieldName + `"`
	keyIdx := strings.Index(text, needle)
	if keyIdx < 0 {
		return nil
	}
	p := keyIdx + len(needle)
	for p < len(text) && text[p] != '[' {
		if text[p] == '{' || text[p] == '"' {
			return nil
		}
		p++
	}
	if p >= len(text) {
		return nil
	}
	p++ // past the '['

	var out [][]byte
	for p < len(text) {
		for p < len(text) && (text[p] == ',' || text[p] == ' ' || text[p] == '\n' || text[p] == '\r' || text[p] == '\t') {
			p++
		}
		if p >= len(text) || text[p] == ']' {
			break
		}
		end, ok := findValueEnd(text, p)
		if !ok {
			break
		}
		elem := strings.TrimSpace(text[p:end])
		if elem != "" && sysjson.Valid([]byte(elem)) {
			out = append(out, []byte(elem))
		}
		p = end
	}
	return out
}

// Truncate shortens s to at most maxLen characters, adding "..." if truncated.
// Handy for embedding response snippets in error messages.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."
	}
	return s[:maxLen-3] + "..."
}

// findValueEnd returns the byte index just past one JSON value starting at start,
// or the position of the enclosing array's closing bracket if the value is
// truncated.
func findValueEnd(text string, start int) (int, bool) {
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(text); i++ {
		c := text[i]
		if escaped {
			escaped = false
			continue
		}
		if inString {
			if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{', '[':
			depth++
		case '}', ']':
			if depth == 0 {
				return i, true
			}
			depth--
			if depth == 0 {
				return i + 1, true
			}
		case ',':
			if depth == 0 {
				return i, true
			}
		}
	}
	return len(text), true
}

// closeTruncatedJSON repairs JSON cut off mid-stream (e.g. by a token limit). It
// truncates at the last top-level comma boundary and closes all open containers.
// Returns the repaired JSON and whether repair was possible.
func closeTruncatedJSON(input string) (string, bool) {
	if len(input) == 0 {
		return input, false
	}

	var stack []byte
	inString := false
	escaped := false
	lastCommaCut := -1

	for i := 0; i < len(input); i++ {
		c := input[i]
		if escaped {
			escaped = false
			continue
		}
		if inString {
			if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			stack = append(stack, '{')
		case '[':
			stack = append(stack, '[')
		case '}':
			if len(stack) > 0 && stack[len(stack)-1] == '{' {
				stack = stack[:len(stack)-1]
			}
		case ']':
			if len(stack) > 0 && stack[len(stack)-1] == '[' {
				stack = stack[:len(stack)-1]
			}
		case ',':
			lastCommaCut = i
		}
	}

	if len(stack) == 0 && !inString {
		return input, false // already complete
	}
	if lastCommaCut < 0 {
		return input, false // nothing safe to truncate to
	}

	truncated := input[:lastCommaCut]

	// Re-scan the truncated portion to recompute the open-container stack.
	stack = stack[:0]
	inString = false
	escaped = false
	for i := 0; i < len(truncated); i++ {
		c := truncated[i]
		if escaped {
			escaped = false
			continue
		}
		if inString {
			if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			stack = append(stack, '{')
		case '[':
			stack = append(stack, '[')
		case '}':
			if len(stack) > 0 && stack[len(stack)-1] == '{' {
				stack = stack[:len(stack)-1]
			}
		case ']':
			if len(stack) > 0 && stack[len(stack)-1] == '[' {
				stack = stack[:len(stack)-1]
			}
		}
	}

	var closing strings.Builder
	closing.WriteString(truncated)
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i] == '{' {
			closing.WriteByte('}')
		} else {
			closing.WriteByte(']')
		}
	}
	return closing.String(), true
}

// errorContext returns a diagnostic snippet around a JSON syntax error's offset,
// with a caret at the offending byte, so prompt tuning can target the real
// failure rather than guessing from the truncated head of the response.
func errorContext(text string, parseErr error) string {
	var syntaxErr *sysjson.SyntaxError
	if !errors.As(parseErr, &syntaxErr) {
		return "Response was: " + Truncate(text, 500)
	}

	const window = 120
	off := int(syntaxErr.Offset)
	if off < 0 || off > len(text) {
		return "Response was: " + Truncate(text, 500)
	}
	start := off - window
	if start < 0 {
		start = 0
	}
	end := off + window
	if end > len(text) {
		end = len(text)
	}
	caret := strings.Repeat(" ", off-start) + "^"
	return fmt.Sprintf("offset=%d, context:\n%s\n%s", off, text[start:end], caret)
}
