# Agent Package

- `DOCS.md`: this directory's file map.
- `brain.go`: composes decision access, browser access, and person analysis into methods callable from an agent loop.
- `runtime.go`: bounded SDK-backed chat/tool loop, native OpenRouter web search, in-memory session continuity, final response generation, and conditional public-URL fetching.
- `runtime_config.go`: chat request/response contracts, runtime/session limits, runtime construction, and system-prompt loading.
- `sessions.go`: expiring session IDs, per-session history, and retained user-selected URLs/allowed hosts.
- `urls.go`: detects and constrains public URLs eligible for OpenRouter web fetching.
- `system_prompt.txt`: embedded default behavior prompt; startup can override it with `DONNA_SYSTEM_PROMPT_FILE`.
- `people.go`: public profile lookup/search contracts, HTTPS URL normalization, unverified candidate results, cross-link identity checks, citation validation, and concise output limits.
- `http_profile_searcher.go`: generic HTTPS adapter for a configured public-profile search provider; accepts name/work/location and only returns URL/platform/name candidate data.
- `tools.go`: provider-neutral tool definitions and strict JSON argument dispatch for agent SDK or HTTP integrations.
- `people_test.go`: verifies candidate lookup, profile identity boundaries, invalid URLs, cited-source enforcement, and output length limits.
- `http_profile_searcher_test.go`: verifies HTTPS enforcement, the narrow request/response contract, and rejection of extra candidate fields.
- `tools_test.go`: verifies tool schemas, dispatch, uncertainty preservation, and optional research-provider behavior.
- `runtime_test.go`: verifies tool-call execution, tool-result replay, chat limits, and prompt customization.
- `urls_test.go`: verifies URL recognition, local-address rejection, and per-message fetch limits.

The browser and analyst are interfaces; no live provider is connected yet.
