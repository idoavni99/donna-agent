# Architecture

## Goals

Keep Donna's backend lean, typed, and easy to change at provider boundaries. Model output is untrusted input: providers return small JSON contracts, while deterministic Go code validates the contracts and applies policy.

## Current shape

```text
HTTP client / agent host
  -> internal/api
      -> internal/agent.Brain
          -> Runtime -> OpenRouter Go SDK
          -> DecisionLayer -> decision.Engine
              -> Classifier / YesNoClassifier -> OpenRouterClassifier
          -> PublicBrowser (optional)
          -> PersonAnalyst (optional)
```

The executable is `cmd/donna`. OpenRouter chat and decision calls use the Go SDK. The application is a modular monolith with no database or background queue.

## Decision layer

The routing contract (`decision.Scores`) has five probabilities: answer directly, structured lookup, semantic search, hybrid search, and clarify. `DecodeScores` rejects missing, extra, null, non-finite, out-of-range, and non-normalized values. `Route` chooses the highest score and falls back to clarify on ties or below the configured threshold.

The agent's yes/no check is deliberately a separate contract (`YesNoScores`). It returns `true` or `false` only when the winning score clears the threshold; ties and low-confidence results serialize as `null`. Never reinterpret retrieval-routing scores as factual yes/no answers.

Provider interfaces (`Classifier` and `YesNoClassifier`) keep the engine separate from the provider. The OpenRouter adapter requests JSON-object output and validates exact schemas in Go.

## Agent brain and public-profile research

`agent.Brain` composes a `DecisionLayer` with optional public-profile search, browser, and analyst providers. `FindPersonProfiles` accepts first and last name plus a work field or broad location and returns up to five HTTPS candidate URLs marked `identity_confirmed: false`. The optional `HTTPProfileSearcher` provides a generic HTTPS adapter; the configured upstream provider must follow the narrow candidate-only JSON contract. Donna does not accept age or appearance as search fields. Research starts from a user-selected profile URL. In chat search, identity checks compare source details with the user's message and the first useful finding; a name match alone is not enough, and Donna asks the user when evidence is insufficient or conflicting. In provider-backed profile research, additional profiles still require an explicit cross-link. A social-media page verifies the platform as the source of its content, not the truth of its claims or the account holder's identity. Analysis contains two or three concise cited summary sentence-items and at most three conversation tips.

Conversation suggestions are scoped to respectful discussion of topics the person chose to publish. Do not add psychological profiling, covert persuasion, or tactics aimed at exploiting personal vulnerabilities. Public availability does not remove the need to honor platform rules and access controls.

`PublicBrowser` and `PersonAnalyst` are interfaces only. They are not connected to the HTTP server and do not fetch or analyze live social profiles yet. Implement these through platform-approved APIs or an authorized browser provider, not an unapproved scraping shortcut.

## Agent-provider integration

`Brain.Tools()` returns provider-neutral tool names, descriptions, and JSON Schemas. A Go host agent can map each definition to its SDK format and call `Brain.InvokeTool()`. Other runtimes can use the optional `GET /v1/tools` and `POST /v1/tools/call` HTTP bridge. `DONNA_AGENT_API_KEY` protects every model-backed HTTP endpoint. This keeps Donna independent of OpenAI, Google, Anthropic, or another SDK. Donna's OpenRouter runtime owns bounded in-memory conversation history; external hosts can manage their own history when calling individual tools. Donna owns capability validation and execution. A decision-only brain requires only a `DecisionLayer`; person-research tools are advertised only when both browser and analyst implementations are supplied.

The OpenRouter SDK drives the bounded chat loop and supplies native web search without a separate Google API key. Donna requests at most three results; the actual backend depends on the selected model/provider and is not necessarily Google. Search results are leads to cross-check, not automatic identity confirmation. Search and fetched page content are untrusted data, never instructions. In-memory sessions retain complete turns up to 24 messages and user-selected public URLs for up to two hours idle; at most 128 sessions are kept. This lets follow-up questions reuse the selected source. Messages with valid public HTTP(S) URLs conditionally receive the SDK's `web_fetch` tool, restricted to supplied or previously selected hosts; private/local hosts are rejected. Sessions are lost on restart. The default prompt is embedded in `internal/agent/system_prompt.txt` and can be replaced at startup with `DONNA_SYSTEM_PROMPT_FILE`. All model-backed API routes require the agent bearer token. Do not expose them over an untrusted network without TLS; keep the default listener on loopback.

## Current scope

- Implemented: OpenRouter SDK client, bounded chat/tool loop, decision API, strict score validation, yes/no check API, authenticated provider-neutral agent tool catalog and dispatcher, generic HTTPS profile-search adapter, profile-link identity checks, citation checks, OpenAPI/Swagger docs, and tests.
- Not implemented: a platform-specific search service, live profile browser, a concrete person analyst, persistent memory, vector storage, file ingestion, and automatic memory distillation.
- `cmd/donna` constructs the OpenRouter client, classifier, decision brain, and chat runtime. The authenticated tool bridge lists route and yes/no tools by default; candidate search is exposed when `DONNA_PROFILE_SEARCH_URL` is set, and profile research requires browser and analyst providers.

## Design rationale

- A single Go service and standard-library HTTP keep the first version operationally small.
- Typed provider interfaces allow vendor changes without moving policy into prompts or transport code.
- Strict, small JSON shapes make model behavior testable and failure modes explicit.
- Uncertain decisions should be represented as uncertainty, not forced into a confident answer.
- Provider-backed profile research uses explicit profile-to-profile links. Chat search checks source details against the user's message and asks for confirmation when those details do not establish identity.
- Provider-specific protocols belong in adapters; stable domain behavior belongs in the internal packages.
