# Changelog

All notable changes to this project are documented here. This project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
