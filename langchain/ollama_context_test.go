package langchain

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaProbeURL(t *testing.T) {
	t.Run("default when nothing set", func(t *testing.T) {
		t.Setenv("OLLAMA_HOST", "")
		if got := ollamaProbeURL(""); got != OllamaServerURL {
			t.Errorf("want %q, got %q", OllamaServerURL, got)
		}
	})

	t.Run("explicit serverURL wins over env", func(t *testing.T) {
		t.Setenv("OLLAMA_HOST", "http://ignored:1")
		if got := ollamaProbeURL("http://host:9999"); got != "http://host:9999" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("bare OLLAMA_HOST gets scheme", func(t *testing.T) {
		t.Setenv("OLLAMA_HOST", "192.168.1.5:11434")
		if got := ollamaProbeURL(""); got != "http://192.168.1.5:11434" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("trailing slash trimmed", func(t *testing.T) {
		t.Setenv("OLLAMA_HOST", "")
		if got := ollamaProbeURL("http://localhost:11434/"); got != "http://localhost:11434" {
			t.Errorf("got %q", got)
		}
	})
}

func TestResolveOllamaNumCtx(t *testing.T) {
	t.Run("override wins without probing", func(t *testing.T) {
		// Unreachable server: an override must not touch the network.
		if got := resolveOllamaNumCtx("any", "http://127.0.0.1:1", 4096); got != 4096 {
			t.Errorf("want 4096, got %d", got)
		}
	})

	t.Run("model max used when under ceiling", func(t *testing.T) {
		srv := showServer(t, `{"model_info":{"gemma.context_length":8192}}`)
		defer srv.Close()
		if got := resolveOllamaNumCtx("gemma", srv.URL, 0); got != 8192 {
			t.Errorf("want 8192, got %d", got)
		}
	})

	t.Run("capped at ceiling for huge-context models", func(t *testing.T) {
		srv := showServer(t, `{"model_info":{"llama.context_length":131072}}`)
		defer srv.Close()
		if got := resolveOllamaNumCtx("llama", srv.URL, 0); got != ollamaNumCtxCeiling {
			t.Errorf("want %d, got %d", ollamaNumCtxCeiling, got)
		}
	})

	t.Run("fallback when server unreachable", func(t *testing.T) {
		if got := resolveOllamaNumCtx("any", "http://127.0.0.1:1", 0); got != ollamaNumCtxFallback {
			t.Errorf("want %d, got %d", ollamaNumCtxFallback, got)
		}
	})
}

func TestOllamaContextLength(t *testing.T) {
	t.Run("parses architecture-prefixed key", func(t *testing.T) {
		srv := showServer(t, `{"model_info":{"qwen2.context_length":32768,"qwen2.block_count":48}}`)
		defer srv.Close()
		n, err := ollamaContextLength("qwen2", srv.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 32768 {
			t.Errorf("want 32768, got %d", n)
		}
	})

	t.Run("errors when no context_length present", func(t *testing.T) {
		srv := showServer(t, `{"model_info":{"general.architecture":"gemma"}}`)
		defer srv.Close()
		if _, err := ollamaContextLength("gemma", srv.URL); err == nil {
			t.Error("want error, got nil")
		}
	})

	t.Run("errors on non-200", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()
		if _, err := ollamaContextLength("missing", srv.URL); err == nil {
			t.Error("want error, got nil")
		}
	})
}

func TestCachedOllamaContextLength(t *testing.T) {
	t.Run("second lookup does not re-probe", func(t *testing.T) {
		var hits int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hits++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model_info":{"gemma.context_length":8192}}`))
		}))
		defer srv.Close()

		// Distinct model name keeps this test's cache entry isolated.
		const model = "cache-test-model-a"
		for i := 0; i < 3; i++ {
			if n, err := cachedOllamaContextLength(model, srv.URL); err != nil || n != 8192 {
				t.Fatalf("call %d: got (%d, %v)", i, n, err)
			}
		}
		if hits != 1 {
			t.Errorf("want 1 probe, got %d", hits)
		}
	})

	t.Run("failures are not cached", func(t *testing.T) {
		// Unreachable → error, and nothing memoized, so a later success works.
		const model = "cache-test-model-b"
		if _, err := cachedOllamaContextLength(model, "http://127.0.0.1:1"); err == nil {
			t.Fatal("want error from unreachable server")
		}
		if _, ok := ollamaCtxLenCache.Load("http://127.0.0.1:1|" + model); ok {
			t.Error("failure should not be cached")
		}
	})
}

func TestJSONNumberToInt(t *testing.T) {
	if n, ok := jsonNumberToInt(float64(4096)); !ok || n != 4096 {
		t.Errorf("float64: got (%d, %v)", n, ok)
	}
	if _, ok := jsonNumberToInt("nope"); ok {
		t.Error("string should not coerce")
	}
}

// showServer returns a test server that replies to POST /api/show with body.
func showServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/show" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}
