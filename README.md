# Donna

Donna is a Python personal-assistant backend built with FastAPI, Pydantic, and Pydantic AI. It uses OpenRouter for chat and decision calls.

## Start here

- [BOOT.md](BOOT.md): Python setup, environment variables, run and verification commands.
- [donna/](donna/): Python application and package map.
- [ARCHITECTURE.md](ARCHITECTURE.md): package boundaries, data flow, and design rationale.
- [DOCS.md](DOCS.md): root files and directory map.
- [AGENTS.md](AGENTS.md): persistent project instructions for coding agents.
- [Swagger UI](http://127.0.0.1:8080/swagger): interactive HTTP API documentation after startup.

The service exposes an authenticated `/v1/tools` bridge and uses Pydantic AI's OpenRouter provider with native web-search capability. Live public-profile browser and analysis providers are not connected; see [ARCHITECTURE.md](ARCHITECTURE.md#current-scope).

`POST /v1/chat` returns a `session_id`; include it on follow-up requests to continue the conversation and research an already selected URL.

Set `DONNA_AGENT_API_KEY` before startup. It protects all model-backed endpoints, including decision and yes/no checks. Keep the listener on loopback or put remote access behind TLS and a trusted network boundary.
