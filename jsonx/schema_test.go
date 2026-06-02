package jsonx

import (
	"testing"
)

type schemaArg struct {
	// Genre to search for.
	Genre    string `json:"genre" jsonschema:"the genre to search for"`
	Limit    int    `json:"limit,omitempty"`
	Verbose  bool   `json:"verbose,omitempty"`
	Required string `json:"required_field"`
}

func TestSchemaMap_Shape(t *testing.T) {
	// A pointer prototype must be dereferenced and produce an object schema.
	m, err := SchemaMap(&schemaArg{})
	if err != nil {
		t.Fatalf("SchemaMap: %v", err)
	}

	if m["type"] != "object" {
		t.Errorf("type = %v, want object", m["type"])
	}

	props, ok := m["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties missing or wrong type: %T", m["properties"])
	}
	for _, want := range []string{"genre", "limit", "verbose", "required_field"} {
		if _, ok := props[want]; !ok {
			t.Errorf("missing property %q in %v", want, props)
		}
	}

	// Required is derived from omitempty: genre and required_field are required;
	// limit and verbose (omitempty) are not.
	required := toStringSet(m["required"])
	for _, want := range []string{"genre", "required_field"} {
		if !required[want] {
			t.Errorf("expected %q to be required; required=%v", want, m["required"])
		}
	}
	for _, notWant := range []string{"limit", "verbose"} {
		if required[notWant] {
			t.Errorf("did not expect %q to be required; required=%v", notWant, m["required"])
		}
	}
}

type nestedArg struct {
	Inner   schemaArg `json:"inner"`
	Tags    []string  `json:"tags"`
	OptTags []string  `json:"opt_tags,omitempty"`
}

// TestSchemaMap_NestedAndSlice locks the intentional shape decisions of the
// google/jsonschema-go switch: nested structs are inlined with their own
// required list, and slices render as a nullable array type.
func TestSchemaMap_NestedAndSlice(t *testing.T) {
	m, err := SchemaMap(&nestedArg{})
	if err != nil {
		t.Fatalf("SchemaMap: %v", err)
	}
	props, ok := m["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties missing: %T", m["properties"])
	}

	// Nested struct is inlined as an object with its own required list.
	inner, ok := props["inner"].(map[string]any)
	if !ok {
		t.Fatalf("inner not an object: %T", props["inner"])
	}
	if inner["type"] != "object" {
		t.Errorf("inner type = %v, want object", inner["type"])
	}
	innerRequired := toStringSet(inner["required"])
	if !innerRequired["genre"] || !innerRequired["required_field"] {
		t.Errorf("nested required missing expected fields: %v", inner["required"])
	}

	// Slice renders as a nullable array.
	tags, ok := props["tags"].(map[string]any)
	if !ok {
		t.Fatalf("tags not an object: %T", props["tags"])
	}
	typeSet := toStringSet(tags["type"])
	if !typeSet["array"] || !typeSet["null"] {
		t.Errorf(`tags type = %v, want ["null","array"]`, tags["type"])
	}

	// omitempty slice is not required; required slice is.
	required := toStringSet(m["required"])
	if !required["tags"] {
		t.Errorf("tags should be required: %v", m["required"])
	}
	if required["opt_tags"] {
		t.Errorf("opt_tags (omitempty) should not be required: %v", m["required"])
	}
}

func TestSchemaMap_ValueAndPointerAgree(t *testing.T) {
	byVal, err := SchemaString(schemaArg{})
	if err != nil {
		t.Fatal(err)
	}
	byPtr, err := SchemaString(&schemaArg{})
	if err != nil {
		t.Fatal(err)
	}
	if byVal != byPtr {
		t.Errorf("value and pointer schemas differ:\n  val: %s\n  ptr: %s", byVal, byPtr)
	}
}

func toStringSet(v any) map[string]bool {
	out := map[string]bool{}
	arr, ok := v.([]any)
	if !ok {
		return out
	}
	for _, x := range arr {
		if s, ok := x.(string); ok {
			out[s] = true
		}
	}
	return out
}
