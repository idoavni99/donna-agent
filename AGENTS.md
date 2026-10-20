# Agent Instructions

Read `README.md`, `ARCHITECTURE.md`, and `BOOT.md` before making broad changes. Each source directory has a `DOCS.md`; update it when files or responsibilities change.

## Project reasoning

- Preserve the lean Go modular-monolith shape. Prefer the standard library and add dependencies only for a concrete need.
- Keep deterministic policy in Go. Treat provider output as untrusted JSON and validate exact schemas at adapter boundaries.
- Keep routing decisions (`Scores`) distinct from yes/no checks (`YesNoScores`). A tie or uncertain check must remain explicitly unknown (`answer: null`).
- Keep vendor protocols behind interfaces/adapters; do not let provider-specific details leak into domain packages.
- Name/context search may return only a short list of unverified public-profile candidates. Person research must start from a user-selected explicit public profile URL. In chat search, compare identity details with the user's message and the first useful finding; a name match alone is not enough. If evidence is insufficient or conflicting, ask the user to verify. In provider-backed profile research, accept additional profiles only when either profile explicitly links to the other. A social-media page verifies where content was published, not the truth of its claims or the account holder's identity. Never use age or appearance as identity evidence.
- Summaries and conversation tips must cite verified public sources and stay concise. Offer respectful conversation ideas, not covert persuasion or vulnerability exploitation.
- Do not claim browser, social-media, memory, vector-store, or file-ingestion functionality is live until an implementation is connected and tested. Use platform-approved APIs or authorized browser access.

## Working and verification

- Read current files before editing, especially when the user reports external changes.
- Keep changes narrow and preserve existing APIs unless the task requires a change.
- Add or update focused tests for behavior changes.
- Run `go test ./...` and `go vet ./...` after Go changes.
- Update relevant `DOCS.md`, `ARCHITECTURE.md`, or `BOOT.md` when contracts or runtime setup change.

## Handoff

- Keep user-facing updates and explanations concise.
- Use the OpenRouter Go SDK already declared in `go.mod`; do not replace it with a handwritten OpenRouter HTTP client.
- OpenRouter chat loop and endpoint OpenAPI/Swagger docs are implemented. Recheck the current worktree and docs before choosing the next integration task.
