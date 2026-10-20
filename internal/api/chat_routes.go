package api

import (
	"errors"
	"net/http"

	"donna/internal/agent"
)

func registerChatRoutes(mux *http.ServeMux, service ChatService, token string) {
	if service != nil {
		mux.Handle("POST /v1/chat", requireBearerToken(token, chat(service)))
	}
}

func chat(service ChatService) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var payload agent.ChatRequest
		if err := decodeRequest(writer, request, &payload); err != nil {
			http.Error(writer, "invalid chat JSON", http.StatusBadRequest)
			return
		}
		result, err := service.Reply(request.Context(), payload)
		switch {
		case errors.Is(err, agent.ErrEmptyChatMessage):
			http.Error(writer, "message is required", http.StatusBadRequest)
		case errors.Is(err, agent.ErrInvalidChatSessionID):
			http.Error(writer, "invalid session_id", http.StatusBadRequest)
		case errors.Is(err, agent.ErrChatSessionNotFound):
			http.Error(writer, "chat session expired or unknown; start a new session", http.StatusNotFound)
		case errors.Is(err, agent.ErrChatSessionCapacity):
			http.Error(writer, "chat sessions at capacity", http.StatusServiceUnavailable)
		case err != nil:
			http.Error(writer, "agent completion failed", http.StatusBadGateway)
		default:
			writeJSON(writer, result)
		}
	}
}
