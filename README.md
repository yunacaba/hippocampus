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
| [`jsonx`](./jsonx) | Reflection-driven JSON serialization and JSON-schema generation (with `proto.Message` support). |
| [`openai`](./openai) | Direct-SDK OpenAI adapter (sets the `user` field). |
| [`anthropic`](./anthropic) | Direct-SDK Anthropic adapter (sets `metadata.user_id`). |
| [`langchain`](./langchain) | langchaingo-backed adapter for Google AI. |
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

Proprietary. Internal use within yunacaba projects.
