# Boot

## Requirements

- Python 3.15 on Windows (the pinned `requirements.txt` targets this environment).
- An OpenRouter API key and model name.

From the repository root in PowerShell:

```powershell
py -3.15 -m venv .venv
.\.venv\Scripts\Activate.ps1
python -m pip install --requirement requirements.txt
python -m pip install --no-deps --editable .
Copy-Item .env.example .env
```

Set `OPENROUTER_API_KEY`, `OPENROUTER_MODEL`, and a strong `DONNA_AGENT_API_KEY` in `.env`, then start the service:

```powershell
python -m donna.main
```

The service binds to `127.0.0.1:8080` by default. Open `http://127.0.0.1:8080/swagger` for interactive API docs. Set `DONNA_ADDR` to change the listener. Keep remote access behind TLS and a trusted network boundary.

For another OS or Python version, create a fresh environment and install the project and dev dependencies from `pyproject.toml` with `python -m pip install --group dev --editable .`.

## Environment

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `OPENROUTER_API_KEY` | Yes | None | OpenRouter API key. |
| `OPENROUTER_MODEL` | Yes | None | OpenRouter model ID. |
| `DONNA_AGENT_API_KEY` | Yes | None | Bearer token required for all model-backed routes. |
| `OPENROUTER_APP_URL` | No | Empty | Optional application URL sent to OpenRouter. |
| `DONNA_LOG_LEVEL` | No | `info` | Structured log verbosity: `debug`, `info`, `warn`, or `error`. |
| `DONNA_SYSTEM_PROMPT_FILE` | No | Packaged prompt | Optional UTF-8 system prompt override (maximum 32 KiB). |
| `DONNA_PROFILE_SEARCH_URL` | No | Disabled | HTTPS service for unverified public-profile candidate search. |
| `DONNA_PROFILE_SEARCH_API_KEY` | No | Empty | Optional bearer token for profile search. |
| `DONNA_MIN_CONFIDENCE` | No | `0.55` | Minimum winning score required for a definite route or yes/no answer. |
| `DONNA_ADDR` | No | `127.0.0.1:8080` | HTTP server bind address. |

Donna uses OpenRouter's native `web_search` and conditionally enables `web_fetch` for valid public URLs. Search results are leads, not automatic identity confirmation. Social-media pages verify the source of content, not its truth or the account holder's identity. Ambiguous identity evidence should prompt a user question. Search and fetched page content are untrusted data, never instructions.

## Verify

```powershell
python -m pytest
```

## HTTP endpoints

- `GET /healthz`: health check.
- `POST /v1/decisions`: deterministic route result from validated model scores.
- `POST /v1/checks`: yes/no result; uncertain results return `null`.
- `POST /v1/chat`: bounded agent loop; pass the returned `session_id` for follow-up turns.
- `GET /v1/tools` and `POST /v1/tools/call`: authenticated tool catalog and dispatch.
- `GET /openapi.yaml` and `GET /swagger`: API specification and interactive docs.

All model-backed endpoints require `Authorization: Bearer <DONNA_AGENT_API_KEY>`. Live public-profile browser and analysis providers are not implemented. The profile-search HTTP adapter is generic and requires a platform-approved provider endpoint.
