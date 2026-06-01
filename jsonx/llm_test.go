package jsonx

import (
	"reflect"
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

func TestExtractJSONObject(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"surrounding prose", `Sure! {"a":1} hope that helps`, `{"a":1}`},
		{"brace in string", `{"a":"}{"}`, `{"a":"}{"}`},
		{"nested", `prefix {"a":{"b":2}} suffix`, `{"a":{"b":2}}`},
		{"no object", `no json here`, `no json here`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractJSONObject(tt.in); got != tt.want {
				t.Errorf("ExtractJSONObject(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
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
	if !contains(err.Error(), "failed to parse JSON") {
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

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
