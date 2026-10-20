package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"donna/internal/agent"
	"donna/internal/decision"
)

type DecisionService interface {
	Decide(context.Context, string) (decision.Result, error)
	Check(context.Context, string, string) (decision.CheckResult, error)
}

type ToolService interface {
	Tools() []agent.ToolDefinition
	InvokeTool(context.Context, string, json.RawMessage) (json.RawMessage, error)
}

type ChatService interface {
	Reply(context.Context, agent.ChatRequest) (agent.ChatResponse, error)
}

type HandlerOptions struct {
	ToolService ToolService
	ChatService ChatService
	ToolToken   string
	Logger      *slog.Logger
}

func NewHandler(service DecisionService, options ...HandlerOptions) http.Handler {
	mux := http.NewServeMux()
	registerDocs(mux)
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	})

	var configured HandlerOptions
	if len(options) > 0 {
		configured = options[0]
	}
	registerDecisionRoutes(mux, service, configured.ToolToken)
	registerToolRoutes(mux, configured.ToolService, configured.ToolToken)
	registerChatRoutes(mux, configured.ChatService, configured.ToolToken)

	logger := configured.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return logRequests(mux, logger)
}

func protectModelRoute(token string, handler http.HandlerFunc) http.Handler {
	if token == "" {
		return handler
	}
	return requireBearerToken(token, handler)
}
