# Agent Instructions

Read `README.md`, `ARCHITECTURE.md`, and `BOOT.md` before broad changes. Each source directory has a `DOCS.md`; update it when files or responsibilities change.

## Project reasoning

- Keep the Python modular monolith lean. Prefer the standard library and add dependencies only for a concrete need.
- Keep deterministic policy in ordinary Python modules. Treat provider output as untrusted and validate exact schemas at adapter boundaries.
- Keep routing scores distinct from yes/no scores. A tie or uncertain check must remain explicitly unknown (`answer: null`).
- Keep vendor protocols behind adapters; do not let provider-specific details leak into domain policy.
- Name/context search may return only a short list of unverified public-profile candidates. Research must start from a user-selected explicit public profile URL. Cross-profile identity requires an explicit link in either direction; matching names, work, location, age, or appearance are not enough. Ask the user to verify when evidence is insufficient.
- Summaries and conversation tips must cite verified public sources and stay concise. Offer respectful conversation ideas, not covert persuasion or vulnerability exploitation.
- Do not claim browser, social-media, memory, vector-store, or file-ingestion functionality is live until an implementation is connected and verified. Use platform-approved APIs or authorized browser access.

## Working and verification

- Read current files before editing, especially when the user reports external changes.
- Keep changes narrow and preserve existing APIs unless the task requires a change.
- Add or update focused tests for behavior changes.
- Run `python -m pytest` after Python changes.
- Update relevant `DOCS.md`, `ARCHITECTURE.md`, or `BOOT.md` when contracts or runtime setup change.

## Handoff

- Keep user-facing updates and explanations concise.
- OpenRouter chat, HTTP routes, and OpenAPI/Swagger docs are implemented. Recheck the current worktree and docs before choosing the next integration task.
