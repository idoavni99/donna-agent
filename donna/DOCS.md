# Python Service

- `__init__.py`: package marker.
- `api.py`: FastAPI app setup, health/decision/check routes, bearer auth, request-size limits, Swagger, and OpenAPI.
- `api_common.py`: shared service/auth contract and strict request JSON parsing for route modules.
- `chat_routes.py`: authenticated `/v1/chat` route and chat error mapping.
- `tool_routes.py`: authenticated tool catalog and dispatch routes.
- `chat.py`: Pydantic AI chat agent, bounded in-memory session history, native web search, and route/check/profile-search tools.
- `constants.py`: named API, session, search, and provider-response resource limits.
- `retry.py`: bounded retries for transient outbound HTTP/network failures, using exponential backoff with full jitter.
- `classifier.py`: Pydantic AI structured-output agents using the OpenRouter provider.
- `config.py`: environment and `.env` settings validation.
- `decision.py`: strict probability contracts and deterministic route/check policy.
- `main.py`: app assembly and Uvicorn entrypoint.
- `openapi.yaml`: packaged HTTP API contract.
- `system_prompt.txt`: default assistant prompt.
- `tools.py`: provider-neutral tool catalog/dispatcher and optional strict HTTPS public-profile search adapter.
- `web.py`: public URL validation and DNS-pinned, host-restricted page fetching for the chat fetch tool.
