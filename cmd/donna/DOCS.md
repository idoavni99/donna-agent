# Donna Command

- `DOCS.md`: this directory's file map.
- `main.go`: loads optional `.env` values, configures structured logging, constructs the OpenRouter SDK client/classifier/chat runtime, an optional public-profile search adapter, and the bearer-protected API; serves the API and performs graceful shutdown.
- `main_test.go`: verifies dotenv loading, shell-variable precedence, optional missing-file behavior, and log-level parsing.