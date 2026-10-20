# Donna

Donna is a Go personal-assistant backend using the OpenRouter Go SDK for chat and decision calls. Chat can fetch user-supplied public URLs through OpenRouter's restricted web-fetch tool. The agent brain also exposes a provider-neutral tool interface.

## Start here

- [BOOT.md](BOOT.md): Go version, environment variables, provider contracts, and run commands.
- [ARCHITECTURE.md](ARCHITECTURE.md): package boundaries, data flow, and design rationale.
- [DOCS.md](DOCS.md): root files and directory map.
- [AGENTS.md](AGENTS.md): persistent project instructions for coding agents.
- [Swagger UI](http://127.0.0.1:8080/swagger): interactive HTTP API documentation after startup.

Each source directory also contains a `DOCS.md` describing its files. Donna uses the OpenRouter Go SDK for its chat loop. Other runtimes can use the authenticated `/v1/tools` HTTP bridge. Live browser/analysis providers and SDK adapters for other model vendors are not included yet; see [ARCHITECTURE.md](ARCHITECTURE.md#current-scope) for the current boundary.

`POST /v1/chat` returns a `session_id`; include it on follow-up requests to continue the conversation and research an already selected URL.

Set `DONNA_AGENT_API_KEY` before startup. It protects all model-backed endpoints, including decision and yes/no checks. Keep the listener on loopback or put remote access behind TLS and a trusted network boundary.
