# Python Port Status

## Completed

- Ported the HTTP API, decision and yes/no policy, OpenRouter chat/tool loop, bounded in-memory sessions, request validation, logging, and OpenAPI/Swagger docs to Python.
- Kept provider integrations behind adapters and exposed deterministic routing/check behavior as tools.
- Added Pydantic AI for structured model calls and the chat agent, with FastAPI and Pydantic for the service boundary.
- Switched startup and setup documentation to the Python service and removed the Go implementation.
- Preserved the identity safeguards: profile candidates stay unverified, research begins from a user-selected URL, cross-profile matching requires an explicit link, and insufficient identity evidence requires user confirmation.

## Remaining validation

- Run `python -m pytest` before merging.
- Perform an OpenRouter live smoke test for multi-turn chat, native search, and URL-fetch restrictions when credentials and model access are available.
- Add formatting, static type, and lint checks when choosing project tooling.

## Not implemented

- Platform-specific profile search, live profile browser, concrete person analyst, persistent memory, vector storage, file ingestion, and automatic memory distillation.
