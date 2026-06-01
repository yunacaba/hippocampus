package hippocampus

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"text/template"

	"github.com/yunacaba/hippocampus/base"
	"github.com/yunacaba/hippocampus/jsonx"
)

// TextPromptTemplate implements text/template-based prompt formatting.
type TextPromptTemplate[TI any, TO any] struct {
	*PromptTemplateBase[TI, TO]
	template  *template.Template
	fields    []PromptField
	sampleArg TI
}

// Matches {{.fieldName}} or {{ .fieldName }} or {{.nested.field}} in prompt templates
var templateVarRegex = regexp.MustCompile(`{{\s*\.([.\w]+)\s*}}`)

// toJSON is a template function that marshals a value to a JSON string.
func toJSON(v interface{}) (string, error) {
	return jsonx.SerializeToString(v)
}

// NewTextPromptTemplate creates a text template.
func NewTextPromptTemplate[TI any, TO any](
	templateText string,
	sampleArg TI,
	sampleResponse TO,
) (*TextPromptTemplate[TI, TO], error) {
	// Create Go template and add our custom toJSON function
	tmpl, err := template.New("prompt").
		Option("missingkey=error").
		Funcs(template.FuncMap{
			"toJSON": toJSON,
		}).
		Parse(templateText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	// Extract field names from template
	matches := templateVarRegex.FindAllStringSubmatch(templateText, -1)
	fields := make([]PromptField, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			fields = append(fields, PromptField{
				Name: match[1],
			})
		}
	}

	// Generate sample response and schema
	sampleResponseStr, err := jsonx.SerializeToString(sampleResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize sample response: %w", err)
	}

	responseSchemaStr, err := jsonx.SchemaString(sampleResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to generate response schema: %w", err)
	}

	return &TextPromptTemplate[TI, TO]{
		PromptTemplateBase: NewPromptTemplateBase[TI, TO](sampleResponseStr, responseSchemaStr, sampleResponse),
		template:           tmpl,
		fields:             fields,
		sampleArg:          sampleArg,
	}, nil
}

// GeneratePrompt implements the PromptTemplate interface using text/template formatting.
func (tpt *TextPromptTemplate[TI, TO]) GeneratePrompt(
	ctx context.Context,
	data TI,
) (base.FormattedPrompt, error) {
	// Parse JSON into map for template
	dataMap, err := jsonx.SerializeToMap(data)
	if err != nil {
		return base.FormattedPrompt{}, fmt.Errorf("failed to convert data to JSON map: %w", err)
	}

	var buf bytes.Buffer
	if err := tpt.template.Execute(&buf, dataMap); err != nil {
		return base.FormattedPrompt{}, fmt.Errorf("failed to format template: %w", err)
	}
	formatted := buf.String()

	return base.FormattedPrompt{
		Prompt:         formatted,
		SampleResponse: tpt.GetSampleResponseString(),
		ResponseSchema: tpt.GetResponseSchema(),
	}, nil
}

// GetFields returns information about the template fields (for debugging/introspection).
func (tpt *TextPromptTemplate[TI, TO]) GetFields() []PromptField {
	return tpt.fields
}
