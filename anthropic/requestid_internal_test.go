package anthropic

import (
	"net/http"
	"testing"
)

// The success path is not expected to hand this a nil response — the SDK
// assigns the capture destination before returning, and a failed request
// returns earlier. The guard exists because the cost of being wrong is
// asymmetric: this code runs on every Anthropic call in the process, so a nil
// dereference here panics an unrelated caller's request rather than losing an
// identifier nobody had.
//
// Tested directly because no public path can produce the case, and an
// untested defensive branch is indistinguishable from an unnecessary one.
func TestResponseRequestIDToleratesAMissingResponse(t *testing.T) {
	if got := responseRequestID(nil); got != "" {
		t.Errorf("nil response: got %q, want empty", got)
	}

	if got := responseRequestID(&http.Response{}); got != "" {
		t.Errorf("response with no headers: got %q, want empty", got)
	}
}

func TestResponseRequestIDIsCaseInsensitive(t *testing.T) {
	// Anthropic sends it lowercase; HTTP header names are case-insensitive and
	// Go canonicalizes on Set, on parse, and on Get.
	//
	// What this pins, precisely: a rewrite to a direct map index keyed by a
	// *non-canonical* literal — resp.Header["request-id"], the spelling the wire
	// uses and therefore the tempting one — returns nothing, because the parsed
	// map is keyed canonically. Indexing with the canonical constant would still
	// work; this is not a claim that Get is the only correct form.
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("request-id", "req_011CQcase")

	if got := responseRequestID(resp); got != "req_011CQcase" {
		t.Errorf("got %q, want req_011CQcase", got)
	}
}
