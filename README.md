# hippocampus

A typesafe Go agent framework. Declare typed inputs and outputs as Go structs;
hippocampus handles prompt formatting, JSON-schema generation, LLM round-trips,
tool dispatch, and result deserialization.

> Status: extraction in progress. APIs are not yet stable.

## Packages

- `hippocampus` (root) — `Agent[TI, TO]`, `Tool[I, O]`, the execution loop, builders.
- `base` — provider-neutral message, tool-call, and call-option types.
- `jsonx` — reflection-driven JSON serialization and JSON-schema generation.
- `openai` — direct-SDK OpenAI model adapter (sets the `user` field).
- `anthropic` — direct-SDK Anthropic model adapter (sets `metadata.user_id`).
- `langchain` — langchaingo-backed adapter (Google AI).
- `agenttest` — mocks and test helpers.
- `sample` — runnable example.

## End-user attribution

Set the end-user account ID once, at your request boundary:

```go
ctx = hippocampus.WithUserID(ctx, userID)
```

OpenAI and Anthropic adapters forward it (`user` / `metadata.user_id`) for
abuse and usage attribution. Adapters that don't support attribution ignore it.

## License

Proprietary. Internal use within yunacaba projects.
