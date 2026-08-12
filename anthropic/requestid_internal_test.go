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
	// Go canonicalizes on both Set and Get. Pinned so a future rewrite to a
	// direct map index — which does not canonicalize — fails here rather than
	// silently reporting nothing in production.
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("request-id", "req_011CQcase")

	if got := responseRequestID(resp); got != "req_011CQcase" {
		t.Errorf("got %q, want req_011CQcase", got)
	}
}
