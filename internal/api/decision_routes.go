package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"donna/internal/decision"
)

const maxRequestBytes = 16 << 10

func registerDecisionRoutes(mux *http.ServeMux, service DecisionService, token string) {
	mux.Handle("POST /v1/decisions", protectModelRoute(token, handleDecision(service)))
	mux.Handle("POST /v1/checks", protectModelRoute(token, handleCheck(service)))
}

func handleDecision(service DecisionService) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var payload struct {
			Query string `json:"query"`
		}
		if err := decodeRequest(writer, request, &payload); err != nil {
			http.Error(writer, "invalid request JSON", http.StatusBadRequest)
			return
		}

		result, err := service.Decide(request.Context(), payload.Query)
		if errors.Is(err, decision.ErrEmptyQuery) {
			http.Error(writer, "query is required", http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(writer, "decision service unavailable", http.StatusBadGateway)
			return
		}
		writeJSON(writer, result)
	}
}

func handleCheck(service DecisionService) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var payload struct {
			Question string `json:"question"`
			Evidence string `json:"evidence"`
		}
		if err := decodeRequest(writer, request, &payload); err != nil {
			http.Error(writer, "invalid request JSON", http.StatusBadRequest)
			return
		}

		result, err := service.Check(request.Context(), payload.Question, payload.Evidence)
		if errors.Is(err, decision.ErrEmptyQuestion) {
			http.Error(writer, "question is required", http.StatusBadRequest)
			return
		}
		if errors.Is(err, decision.ErrCheckUnavailable) {
			http.Error(writer, "yes/no checks are not configured", http.StatusServiceUnavailable)
			return
		}
		if err != nil {
			http.Error(writer, "decision service unavailable", http.StatusBadGateway)
			return
		}
		writeJSON(writer, result)
	}
}

func decodeRequest(writer http.ResponseWriter, request *http.Request, target any) error {
	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request must contain one JSON object")
	}
	return nil
}

func writeJSON(writer http.ResponseWriter, value any) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(value)
}
