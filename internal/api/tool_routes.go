package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"donna/internal/agent"
	"donna/internal/decision"
)

func registerToolRoutes(mux *http.ServeMux, service ToolService, token string) {
	if service == nil {
		return
	}
	mux.Handle("GET /v1/tools", requireBearerToken(token, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, service.Tools())
	})))
	mux.Handle("POST /v1/tools/call", requireBearerToken(token, callTool(service)))
}

func callTool(service ToolService) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var payload struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := decodeRequest(writer, request, &payload); err != nil || payload.Name == "" || len(payload.Arguments) == 0 {
			http.Error(writer, "tool call requires one name and arguments object", http.StatusBadRequest)
			return
		}

		result, err := service.InvokeTool(request.Context(), payload.Name, payload.Arguments)
		if mapToolError(writer, err) {
			return
		}
		if !json.Valid(result) {
			http.Error(writer, "tool returned invalid JSON", http.StatusInternalServerError)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(result)
	}
}

func mapToolError(writer http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, agent.ErrUnknownTool):
		http.Error(writer, "unknown tool", http.StatusNotFound)
	case errors.Is(err, agent.ErrResearchUnavailable):
		http.Error(writer, "person research is not configured", http.StatusServiceUnavailable)
	case errors.Is(err, agent.ErrProfileSearchUnavailable):
		http.Error(writer, "public profile search is not configured", http.StatusServiceUnavailable)
	case errors.Is(err, agent.ErrInvalidToolInput), errors.Is(err, decision.ErrEmptyQuery), errors.Is(err, decision.ErrEmptyQuestion), errors.Is(err, agent.ErrInvalidProfileURL), errors.Is(err, agent.ErrTooManyProfiles), errors.Is(err, agent.ErrInvalidPersonLookup), errors.Is(err, agent.ErrTooManySearchResults):
		http.Error(writer, "invalid tool arguments", http.StatusBadRequest)
	case err != nil:
		http.Error(writer, "tool execution failed", http.StatusBadGateway)
	default:
		return false
	}
	return true
}
