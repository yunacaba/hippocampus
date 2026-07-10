package langchain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Ollama's default context window is 2048 tokens. That silently truncates any
// prompt longer than a couple thousand tokens — the model then sees only a
// fragment, with no error. This file sizes num_ctx to the model instead:
// query /api/show for the model's trained context length and use it, capped so
// a huge-context model doesn't force a multi-GB KV-cache allocation.

const (
	// ollamaNumCtxCeiling caps the auto-detected context window. Leaves ample
	// headroom for realistic prompts without over-allocating the KV cache on a
	// model that advertises 128k+ context. Callers wanting more can pass an
	// explicit value via WithOllamaNumCtx.
	ollamaNumCtxCeiling = 32768

	// ollamaNumCtxFallback is used when the model's context length can't be
	// read (Ollama unreachable, unexpected /api/show shape). Well above the
	// 2048 default that truncates.
	ollamaNumCtxFallback = 8192

	// ollamaShowTimeout bounds the /api/show probe so a slow or missing daemon
	// can't stall model construction.
	ollamaShowTimeout = 3 * time.Second
)

// ollamaCtxLenCache memoizes successful /api/show context-length lookups,
// keyed by "serverURL|model". A model's context length is fixed for a running
// Ollama server, and callers construct a provider per request, so without this
// every call would re-probe. Failures are not cached, so a server that was
// down recovers on the next call.
var ollamaCtxLenCache sync.Map

// resolveOllamaNumCtx picks the context window for a model. An explicit
// override (> 0) wins; otherwise the model's trained context length from
// /api/show, capped at ollamaNumCtxCeiling; otherwise ollamaNumCtxFallback.
func resolveOllamaNumCtx(model, serverURL string, override int) int {
	if override > 0 {
		return override
	}
	maxCtx, err := cachedOllamaContextLength(model, serverURL)
	if err != nil || maxCtx <= 0 {
		return ollamaNumCtxFallback
	}
	if maxCtx > ollamaNumCtxCeiling {
		return ollamaNumCtxCeiling
	}
	return maxCtx
}

// cachedOllamaContextLength returns the model's context length, memoizing
// successful lookups. Only successes are cached (see ollamaCtxLenCache).
func cachedOllamaContextLength(model, serverURL string) (int, error) {
	key := serverURL + "|" + model
	if v, ok := ollamaCtxLenCache.Load(key); ok {
		return v.(int), nil
	}
	n, err := ollamaContextLength(model, serverURL)
	if err != nil {
		return 0, err
	}
	ollamaCtxLenCache.Store(key, n)
	return n, nil
}

// ollamaProbeURL resolves the base URL for the /api/show probe: the explicit
// serverURL, else OLLAMA_HOST (accepting a bare "host:port"), else the local
// default. Mirrors how the Ollama client resolves its host, so the probe hits
// the same server the model will.
func ollamaProbeURL(serverURL string) string {
	raw := serverURL
	if raw == "" {
		raw = os.Getenv("OLLAMA_HOST")
	}
	if raw == "" {
		return OllamaServerURL
	}
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "http://" + raw
	}
	return strings.TrimRight(raw, "/")
}

// ollamaContextLength queries Ollama's /api/show for the model's trained
// context length. In model_info the field is architecture-prefixed
// (e.g. "llama.context_length", "gemma.context_length"), so we scan for the
// first key ending in ".context_length".
func ollamaContextLength(model, serverURL string) (int, error) {
	reqBody, err := json.Marshal(map[string]string{"model": model})
	if err != nil {
		return 0, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), ollamaShowTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ollamaProbeURL(serverURL)+"/api/show", bytes.NewReader(reqBody))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("/api/show returned %s", resp.Status)
	}

	var show struct {
		ModelInfo map[string]any `json:"model_info"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&show); err != nil {
		return 0, err
	}

	for k, v := range show.ModelInfo {
		if strings.HasSuffix(k, ".context_length") {
			if n, ok := jsonNumberToInt(v); ok {
				return n, nil
			}
		}
	}
	return 0, fmt.Errorf("no *.context_length in model_info")
}

// jsonNumberToInt coerces a JSON-decoded numeric value (float64 by default) to
// an int.
func jsonNumberToInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i), true
		}
	}
	return 0, false
}
