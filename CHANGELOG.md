# Changelog

All notable changes to this project are documented here. This project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- **`langchain.NewOllamaProvider` now honors `OLLAMA_HOST`.** Passing an empty
  `serverURL` no longer pins the client to `localhost`; it leaves the server URL
  unset so langchaingo resolves the host from the `OLLAMA_HOST` environment
  variable (falling back to `127.0.0.1:11434`), matching plain `ollama.New()`.
  An explicit `serverURL` is still honored verbatim. (Regression from the
  initial provider in 0.5.0, which always set the server URL.)

## [0.5.0] - 2026-06-02

### Added

- **Native Ollama provider in the `langchain` package.**
  `langchain.NewOllamaProvider(serverURL)` builds models backed by langchaingo's
  native Ollama client (the `/api/chat` API, default `http://localhost:11434`,
  no API key). Unlike `openaicompat.Ollama` (which uses Ollama's
  OpenAI-compatible `/v1` endpoint), this provider lets langchaingo control
  extended thinking: `WithThinking` maps to `WithThinkingMode(Auto)` → Ollama's
  native `think` toggle; the default maps to `WithThinkingMode(None)`.
  langchaingo's reasoning detection is name-based (`deepseek-r1`, `qwq`,
  `reasoning`, `thinking`) and cannot send an explicit "off". Model names are
  arbitrary; schema enforcement is not advertised (relies on the `jsonx`
  parser). New `LLMVendorOllama` labels these models.

- **Prompt caching and extended thinking / reasoning.** New `WithPromptCaching`,
  `WithThinking`, and `WithThinkingBudget` call options:
  - Anthropic → `cache_control` on the system prompt; extended thinking via
    `thinking` (budget defaulted/clamped >= 1024, `max_tokens` raised above the
    budget, temperature suppressed as the API requires).
  - OpenAI → reasoning effort (`reasoning_effort`) for thinking; prompt caching
    is automatic, so `WithPromptCaching` is a no-op.
  - Google AI / Ollama (langchaingo) → thinking via `WithThinkingMode`
    (Ollama only); prompt caching not supported.

### Changed

- Bumped `github.com/tmc/langchaingo` from `v0.1.14-pre.3` to `v0.1.14` (stable),
  which introduces the `WithThinkingMode` call option.

## [0.4.0] - 2026-06-02

### Added

- **`openaicompat` package** — providers for OpenAI-compatible servers (local
  runtimes like Ollama and LM Studio). `NewProvider(baseURL, …)` plus `Ollama()`
  / `LMStudio()` presets: no API key required, arbitrary model names, and
  response-schema enforcement off by default (`WithResponseSchemaSupport(true)`
  to enable for servers that constrain output). Thin wrapper over the `openai`
  adapter.

## [0.3.0] - 2026-06-02

### Added

- **Structured outputs.** New `WithResponseSchema` / `WithStrictResponseSchema`
  call options and a `base.CallOptions.ResponseSchema`. The agent builder's
  `SetStructuredOutput(true)` derives the JSON Schema from the output type `O`
  and sends it to the model so it returns schema-conformant JSON:
  - OpenAI → `response_format: json_schema` (precedes plain JSON mode).
  - Anthropic → a forced synthetic output tool (only when the call has no other
    tools); the tool's input is lifted back into the response content.
  - Google AI (langchaingo) → no schema enforcement; relies on prompt guidance
    and the `jsonx` cleaner (see issue #2).

- `jsonx` LLM-tolerant parsing, ported from Story Builder's `aiutil`:
  `DeserializeLLM[T]` / `DeserializeAnyLLM` (clean → unmarshal → `jsonrepair` →
  truncation-close fallback, reporting whether repair was needed), plus
  `CleanLLMJSON`, `CleanMarkdownBlock`, `ExtractJSONValue` (object or array),
  `SalvageArrayElements`, and `Truncate`. The agent now parses model output
  through this path, tolerating markdown fences, surrounding prose, and
  truncation.

### Changed

- Minimum Go version is now **1.26** (transitively required by
  `github.com/kaptinlin/jsonrepair`).
- **Structured output is now on by default** for agents. The schema is attached
  only when the model reports it can enforce one (new `ResponseSchemaCapable`
  interface — OpenAI/Anthropic report true, Google AI false) and the output type
  is object-rooted; otherwise the agent falls back to prompt guidance + the
  tolerant parser. Opt out with `SetStructuredOutput(false)`.
- Provider `Model()` now accepts **arbitrary model-name strings** (using the name
  as the wire model id), rejecting only a model that is a *known other vendor*.
  This lets new/unreleased models — and OpenAI-compatible local models — be used
  without a predefined constant.
- The OpenAI provider gained `WithResponseSchemaSupport(bool)` (default true) so
  OpenAI-compatible endpoints that don't honor `response_format: json_schema`
  can declare it, gating structured output off for them.
- `jsonx` schema generation (`SchemaBytes`/`SchemaString`/`SchemaMap`) now uses
  `github.com/google/jsonschema-go` instead of `swaggest/jsonschema-go` — the
  package the Go MCP SDK standardizes on, which also provides validation for
  later use. Public signatures are unchanged.

  Behavior changes for consumers:
  - **Required-ness** is derived from `omitempty`/`omitzero` (matching
    `encoding/json`) rather than a `required:"true"` tag.
  - **swaggest-family struct tags are no longer honored** — `required:"true"`,
    `minimum`/`maximum` (`min`/`max`), and similar are ignored. Only `json`,
    `omitempty`/`omitzero`, and `jsonschema:"..."` (descriptions) are read.
    Structs relying on those tags should migrate (`omitempty` for optional
    fields; range constraints have no struct-tag equivalent).
  - Generated object schemas now set `additionalProperties: false` and inline
    nested structs rather than emitting `$defs`/`$ref`. Slices render as
    nullable (`"type": ["null", "array"]`).

### Removed

- `DeserializeFromPartialString` / `DeserializeAnyFromPartialString` and the
  `github.com/blaze2305/partial-json-parser` dependency. Superseded by the
  strictly-more-capable `DeserializeLLM` / `DeserializeAnyLLM`.

## [0.2.1] - 2026-06-01

### Added

- Apache License 2.0 and a `NOTICE` file; the project is now public OSS.
  Updated the README license section accordingly. No code changes.

## [0.2.0] - 2026-06-01

Code-review follow-up: correctness fixes, an adapter refactor, and fuller
CallOptions coverage. Contains API-visible changes (see Removed), hence a minor
bump.

### Fixed

- **Tool-call pairing**: results are now paired to their originating call
  positionally instead of by `ToolCallID`. Providers that omit IDs (Google AI)
  or reuse them previously collapsed parallel calls, corrupting the conversation
  history sent on the next turn.
- **Request aliasing**: each recorded `details.ModelRequests` entry is now a
  clone, so it can't be mutated (or data-raced) by later appends to the agent's
  working message slice.
- **OpenAI streaming token usage**: streamed calls now request `include_usage`,
  so token counts are reported instead of zero.
- **Streaming TTFT**: time-to-first-token is recorded on any first delta
  (content or tool-call), so tool-only streamed turns are measured.
- **Anthropic JSON mode**: `WithJSONMode` is honored via a system instruction
  instead of being silently dropped (the API has no JSON-mode flag).
- **EnvKeyProvider**: a nil vendor now returns an error instead of panicking.

### Added

- `RunModelGenerate` and `MarkFirstToken` helpers for implementing `base.Model`
  adapters; the three bundled adapters now share this scaffolding.
- `WithToolChoice` and `WithStopWords` are now honored across all adapters
  (`ToolChoice`: auto/required/none/named; OpenAI also gained stop-word
  support).

### Changed

- Tool input JSON Schema is computed once at `NewTool` time, not per call.

### Removed

- `ModelCallResponse.FuncCall` (was never read; use `ToolCalls`).
- `WithMetadata` / `CallOptions.Metadata` (no meaningful provider-neutral
  semantics; was silently ignored by every adapter).

## [0.1.1] - 2026-06-01

### Changed

- Reformatted to satisfy gofumpt v0.10.0 and pinned the CI gofumpt version so
  `@latest` drift cannot fail the build. No API or behavior changes.

## [0.1.0] - 2026-06-01

Initial release. Extracted from Carmen's in-tree agent framework into a
standalone module with provider-neutral types and direct-SDK adapters.

### Added

- **Core framework** (`hippocampus`): generic `Agent[TI, TO]`, typed
  `Tool[I, O]`, staged fluent builder, the model/tool execution loop,
  `ToolDelegate` (accept/reject/modify), `AgentObserver`, prompt templates, and
  debug logging.
- **Provider-neutral types** (`base`): `Message`/`ContentPart` (a sealed sum
  type), `ModelToolCall`, `CallOption`/`CallOptions`, `ToolSpec`, and the model
  interfaces — no langchaingo in the public API.
- **`jsonx`**: reflection-driven serialization and JSON-schema generation,
  including a `proto.Message` (protojson) special case.
- **End-user attribution**: `WithUserID(ctx, id)` plumbed through context;
  forwarded as OpenAI `user` and Anthropic `metadata.user_id`.
- **Adapters**: direct-SDK `openai` (openai-go v2) and `anthropic`
  (anthropic-sdk-go v1.46.0); `langchain` (langchaingo) for Google AI. Each
  supports streaming with time-to-first-token metrics.
- **Pluggable infrastructure**: `Tracer`/`Span` (default `NoopTracer`) and
  `KeyProvider` (default `EnvKeyProvider`).
- **`agenttest`**: `MockModel`, `MockModelProvider`, `MockAgentObserver`,
  `MockToolDelegate`.
- **`sample`**: a runnable, typed agent demo.

### Notes

- Model constants (`OpenAIGPT4OMini`, `AnthropicClaudeHaiku45`,
  `GoogleAIGemini25Flash`, …) and `LLMType`/`LLMVendor` live in the root
  package, not the adapter subpackages, to keep the vendor-dispatch logic free
  of import cycles.
- Real-API tests are gated behind the `llm` build tag; the default test run is
  hermetic.
