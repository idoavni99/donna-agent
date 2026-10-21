# Python Service

- `__init__.py`: package marker.
- `api.py`: FastAPI health, decision/check/chat/tool, Swagger, and OpenAPI routes; bearer auth and request-size limits.
- `chat.py`: Pydantic AI chat agent, bounded in-memory sessions, native web search, and route/check/profile-search tools.
- `classifier.py`: Pydantic AI structured-output agents using the OpenRouter provider.
- `config.py`: environment and `.env` settings validation.
- `decision.py`: strict probability contracts and deterministic route/check policy.
- `main.py`: app assembly and Uvicorn entrypoint.
- `openapi.yaml`: packaged HTTP API contract.
- `system_prompt.txt`: default assistant prompt.
- `tools.py`: provider-neutral tool catalog/dispatcher and optional strict HTTPS public-profile search adapter.
- `web.py`: public URL validation and DNS-pinned, host-restricted page fetching for the chat fetch tool.
