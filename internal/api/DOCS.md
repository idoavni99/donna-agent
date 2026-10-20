# API Package

- `DOCS.md`: this directory's file map.
- `handler.go`: shared route registration, handler interfaces, and request logging setup.
- `decision_routes.go`: decision and yes/no endpoints, strict request decoding, and JSON responses.
- `tool_routes.go`: authenticated tool catalog and invocation endpoints.
- `chat_routes.go`: authenticated chat endpoint and session error mapping.
- `auth.go`: bearer-token middleware shared by model-backed routes.
- `swagger.go`: serves the embedded OpenAPI spec and Swagger UI page.
- `openapi.yaml`: OpenAPI 3.1 descriptions for every HTTP endpoint.
- `decision_test.go`: endpoint response, malformed request, uncertainty, error mapping, tool authentication/dispatch, chat, and docs tests.
