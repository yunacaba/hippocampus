package jsonx

import (
	"reflect"
	"strings"
	"testing"
)

func TestCleanMarkdownBlock(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no fence", `{"a":1}`, `{"a":1}`},
		{"json fence", "```json\n{\"a\":1}\n```", `{"a":1}`},
		{"bare fence", "```\n{\"a\":1}\n```", `{"a":1}`},
		{"truncated fence", "```json\n{\"a\":1}", `{"a":1}`},
		{"fence after prose", "Here you go:\n```json\n{\"a\":1}\n```", `{"a":1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CleanMarkdownBlock(tt.in); got != tt.want {
				t.Errorf("CleanMarkdownBlock(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestExtractJSONValue(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"surrounding prose", `Sure! {"a":1} hope that helps`, `{"a":1}`},
		{"brace in string", `{"a":"}{"}`, `{"a":"}{"}`},
		{"nested object", `prefix {"a":{"b":2}} suffix`, `{"a":{"b":2}}`},
		{"top-level array of objects", `[{"a":1},{"b":2}]`, `[{"a":1},{"b":2}]`},
		{"array with prose", `Results: [1,2,3] done`, `[1,2,3]`},
		{"object before array", `{"a":[1,2]}`, `{"a":[1,2]}`},
		{"bracket in array string", `["a]b","c"]`, `["a]b","c"]`},
		{"no json", `no json here`, `no json here`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractJSONValue(tt.in); got != tt.want {
				t.Errorf("ExtractJSONValue(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDeserializeLLM_TopLevelArray(t *testing.T) {
	// An agent whose output type is a slice and whose model returns a bare,
	// fenced array must round-trip — the object-only extractor used to mangle this.
	got, repaired, err := DeserializeLLM("```json\n"+`[{"name":"a","genres":["x"]},{"name":"b","genres":[]}]`+"\n```", &[]sample{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if repaired {
		t.Error("clean array should not need repair")
	}
	if len(*got) != 2 || (*got)[0].Name != "a" || (*got)[1].Name != "b" {
		t.Errorf("got %#v", got)
	}
}

type sample struct {
	Name   string   `json:"name"`
	Genres []string `json:"genres"`
}

func TestDeserializeLLM_CleanInput(t *testing.T) {
	got, repaired, err := DeserializeLLM(`Here:
`+"```json\n"+`{"name":"x","genres":["a","b"]}`+"\n```", &sample{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if repaired {
		t.Error("clean input should not need repair")
	}
	if got.Name != "x" || !reflect.DeepEqual(got.Genres, []string{"a", "b"}) {
		t.Errorf("got %#v", got)
	}
}

func TestDeserializeLLM_RepairsTrailingComma(t *testing.T) {
	got, repaired, err := DeserializeLLM(`{"name":"x","genres":["a","b",]}`, &sample{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !repaired {
		t.Error("expected repaired=true for trailing comma")
	}
	if !reflect.DeepEqual(got.Genres, []string{"a", "b"}) {
		t.Errorf("got %#v", got)
	}
}

func TestDeserializeLLM_ClosesTruncated(t *testing.T) {
	// Truncated mid-array after a complete element + comma; closeTruncatedJSON
	// should cut at the last comma and close the open containers.
	got, repaired, err := DeserializeLLM(`{"name":"x","genres":["a","b",`, &sample{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !repaired {
		t.Error("expected repaired=true for truncated input")
	}
	if got.Name != "x" || !reflect.DeepEqual(got.Genres, []string{"a", "b"}) {
		t.Errorf("got %#v", got)
	}
}

func TestDeserializeLLM_UnrecoverableErrorHasContext(t *testing.T) {
	_, _, err := DeserializeLLM(`not json at all`, &sample{})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "failed to parse JSON") {
		t.Errorf("error missing prefix: %v", err)
	}
}

func TestSalvageArrayElements(t *testing.T) {
	// One malformed middle element should not discard the whole array.
	text := `{"items":[{"id":1},{"id":,},{"id":3}]}`
	elems := SalvageArrayElements(text, "items")
	if len(elems) != 2 {
		t.Fatalf("want 2 salvaged elements, got %d: %q", len(elems), elems)
	}
	if string(elems[0]) != `{"id":1}` || string(elems[1]) != `{"id":3}` {
		t.Errorf("unexpected salvage: %q", elems)
	}
}

func TestTruncate(t *testing.T) {
	if got := Truncate("hello world", 8); got != "hello..." {
		t.Errorf("got %q", got)
	}
	if got := Truncate("hi", 8); got != "hi" {
		t.Errorf("got %q", got)
	}
}
