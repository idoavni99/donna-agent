# Decision Package

- `DOCS.md`: this directory's file map.
- `decision.go`: route action and probability types, exact JSON decoding, score validation, and deterministic highest-score routing.
- `engine.go`: provider-independent orchestration for retrieval decisions and yes/no checks.
- `openrouter_classifier.go`: OpenRouter SDK adapter for structured route and yes/no scores.
- `yesno.go`: separate yes/no probability contract, strict decoder, validation, and explicit uncertain result.
- `http_classifier.go`: HTTP adapter for route and yes/no requests, including response size limits and optional bearer authentication.
- `decision_test.go`: score decoding, routing, confidence, and engine behavior tests.
- `http_classifier_test.go`: provider adapter request/response tests.
- `openrouter_classifier_test.go`: structured OpenRouter classifier behavior tests.