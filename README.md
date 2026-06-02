# hippocampus

A typesafe Go agent framework. Declare your inputs and outputs as Go structs;
hippocampus handles prompt formatting, JSON-schema generation, LLM round-trips,
tool dispatch, and result deserialization — so callers write types, not glue.

```go
agent, _ := hippocampus.NewAgentWithTemplateText(
    "Recommend shows similar to:\n{{range .favorite_shows}}- {{.}}\n{{end}}",
    &RecommendationRequest{},
    &RecommendationResponse{},
).
    SetName("recommender").
    SetModel(openai.NewProvider(hippocampus.EnvKeyProvider{}), hippocampus.OpenAIGPT4OMini).
    AddTool(hippocampus.NewTool("catalog_search", "Search the catalog.", searchCatalog, &CatalogQuery{}, &CatalogResult{})).
    Build()

resp, err := agent.Execute(ctx, &RecommendationRequest{FavoriteShows: []string{"Star Trek"}}, nil)
// resp is a *RecommendationResponse — fully typed, no manual JSON.
```

A complete, runnable version lives in [`sample/`](./sample); run it with
`OPENAI_API_KEY=sk-... go run ./sample`.

## Core concepts

- **`Agent[TI, TO]`** — generic over a typed input and a typed output. The agent
  formats a prompt from `TI`, runs the model/tool loop, and deserializes the
  model's final answer into `TO`.
- **`Tool[I, O]`** — a tool with typed input and output. Its JSON Schema is
  generated from `I` by reflection; arguments and results are (de)serialized for
  you.
- **`ToolDelegate`** — intercept tool calls before execution to accept, reject,
  or modify them (human-in-the-loop / policy enforcement).
- **`AgentObserver`** — observe the execution (LLM calls, tool calls, errors),
  e.g. to persist a transcript. Observers may be called concurrently.

## Packages

| Package | Purpose |
|---|---|
| `hippocampus` (root) | `Agent[TI, TO]`, `Tool[I, O]`, builder, execution loop, model constants. |
| [`base`](./base) | Provider-neutral message, tool-call, and call-option types. |
| [`jsonx`](./jsonx) | Reflection-driven JSON serialization, JSON-schema generation (with `proto.Message` support), and LLM-tolerant parsing (`DeserializeLLM`: strips fences/prose, repairs malformed/truncated JSON). |
| [`openai`](./openai) | Direct-SDK OpenAI adapter (sets the `user` field). |
| [`anthropic`](./anthropic) | Direct-SDK Anthropic adapter (sets `metadata.user_id`). |
| [`langchain`](./langchain) | langchaingo-backed adapter for Google AI. |
| [`openaicompat`](./openaicompat) | Providers for OpenAI-compatible servers — local runtimes like Ollama and LM Studio. |
| [`agenttest`](./agenttest) | Mocks: `MockModel`, `MockModelProvider`, `MockAgentObserver`, `MockToolDelegate`. |
| [`sample`](./sample) | Runnable example. |

## End-user attribution

Set the end-user account ID once, at your request boundary (e.g. a gRPC auth
interceptor):

```go
ctx = hippocampus.WithUserID(ctx, userID)
```

Everything downstream — every `Execute`, tool call, and `Model.Generate` — picks
it up automatically:

| Vendor | Field set |
|---|---|
| OpenAI | `user` (top-level on the chat completion request) |
| Anthropic | `metadata.user_id` |
| Google AI | not supported by the API; ignored |

## Call options

Per-call options passed to `Execute` (or set by the agent) are translated by
each adapter to its SDK: `WithTemperature`, `WithMaxTokens`, `WithTopP`,
`WithStopWords`, `WithJSONMode`, `WithTools`, and `WithToolChoice`
(`"auto"`/`"required"`/`"none"`/a tool name). Anthropic has no JSON-mode flag,
so `WithJSONMode` is applied as a system instruction there.

### Structured outputs

Agents derive a JSON Schema from the output type `O` and ask the model to
conform to it — **on by default**. OpenAI uses `response_format: json_schema`,
Anthropic uses a forced output tool (when the call has no other tools). The
schema is attached only when the model can enforce it (`ResponseSchemaCapable`)
and `O` is object-rooted; otherwise the agent relies on prompt guidance plus the
tolerant `jsonx` parser. Opt out with `SetStructuredOutput(false)`, or set a
schema per call with `WithResponseSchema`.

Model names are not restricted to the predefined constants — any string is
accepted (used as the wire model id), so new models and OpenAI-compatible local
servers work too. For a local server that doesn't honor `response_format`, use
`openai.WithResponseSchemaSupport(false)` so the agent falls back gracefully.

### Local models (Ollama, LM Studio)

The [`openaicompat`](./openaicompat) package wraps the OpenAI adapter for
OpenAI-compatible servers — no API key required, arbitrary model names, and
schema enforcement off by default (enable with `WithResponseSchemaSupport(true)`
for servers that constrain output):

```go
provider := openaicompat.Ollama() // or LMStudio(), or NewProvider("http://host/v1")
agent, _ := hippocampus.NewAgentWithTemplateText(tmpl, &Req{}, &Resp{}).
    SetName("local").
    SetModel(provider, hippocampus.LLMType("qwen2.5")).
    Build()
```

## Tracing and keys

- **Tracing** is behind the `Tracer`/`Span` interfaces; the default is
  `NoopTracer`. Provide your own via `builder.SetTracer(...)` or
  `provider.WithTracer(...)` to emit spans (e.g. an OpenTelemetry adapter).
- **API keys** come from a `KeyProvider`. `EnvKeyProvider` (the default) reads
  `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, and `GOOGLE_AI_API_KEY`.

## Testing

`go test ./...` is hermetic and requires no API keys. Real-API integration tests
are gated behind the `llm` build tag:

```sh
OPENAI_API_KEY=...   go test -tags=llm ./openai/...
ANTHROPIC_API_KEY=... go test -tags=llm ./anthropic/...
GOOGLE_AI_API_KEY=... go test -tags=llm ./langchain/...
```

## License

Licensed under the [Apache License 2.0](./LICENSE). See [NOTICE](./NOTICE) for
attribution.
