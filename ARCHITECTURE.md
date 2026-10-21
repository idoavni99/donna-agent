# Architecture

## Goals

Keep Donna lean, typed, and easy to change at provider boundaries. Provider output is untrusted; validate exact schemas before applying deterministic policy. The service is a Python modular monolith with no database or background queue.

## Current shape

```text
HTTP client / agent host
  -> FastAPI routes
      -> deterministic decision policy
      -> Pydantic AI OpenRouter agents
      -> provider-neutral tools
          -> optional HTTPS profile candidate search
          -> restricted public URL fetch
```

`donna.main` creates the FastAPI app and starts Uvicorn. Routing and yes/no decisions use separate score contracts. A tie or low-confidence answer remains unknown (`null`). Pydantic AI handles chat and structured provider calls; ordinary Python code owns deterministic policy and request validation.

Chat uses native OpenRouter web search and an in-memory bounded session store. Sessions retain complete turns up to 24 messages and 128 KiB of serialized history, expire after two hours idle, and are capped at 128. That caps serialized history at 16 MiB across all sessions; sessions are lost on restart. A single turn larger than the per-session budget is rejected instead of retained. Public URL fetching is restricted to user-supplied or selected public hosts; local and private hosts are rejected.

Chat transcripts are short-lived operational session state. Redis is a future fit if sessions need to survive restarts or be shared across service instances. A vector database serves a separate purpose: retrieving long-term semantic memories, not storing complete chat transcripts.

## Identity and source handling

Name/context search returns only a short list of unverified public-profile candidates. Research starts from a user-selected explicit public profile URL. Cross-profile identity requires an explicit link in either direction; matching names, work, location, age, or appearance are not enough. If details from the user's message and first useful finding do not establish identity, ask the user to verify.

A social-media page verifies where content appeared, not the truth of its claims or who controls the account. Summaries cite verified public sources and stay concise. Conversation ideas should be respectful and based on topics the person chose to publish.

## Current scope

- Implemented: authenticated health, decision, check, chat, and tool routes; strict score validation; session limits; request logging with redaction; OpenAPI and Swagger; OpenRouter structured calls and chat; native web search; restricted URL fetching; optional generic HTTPS profile-candidate search.
- Not implemented: persistent memory, vector storage, file ingestion, automatic memory distillation, and execution through the Google Workspace MCP toolset. Google OAuth and approval proposals are available when configured; credentials and approvals are intentionally in-memory until a database is added.

Provider-specific protocols belong in adapters. Search results and fetched page content are untrusted data, never instructions. Keep the default listener on loopback. Protect remote access with TLS and a trusted network boundary.
