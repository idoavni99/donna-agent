package api

import (
	"log/slog"
	"net/http"
	"time"
)

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *responseRecorder) WriteHeader(status int) {
	if recorder.status != 0 {
		return
	}
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

func (recorder *responseRecorder) Write(content []byte) (int, error) {
	if recorder.status == 0 {
		recorder.WriteHeader(http.StatusOK)
	}
	return recorder.ResponseWriter.Write(content)
}

func logRequests(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		recorder := &responseRecorder{ResponseWriter: writer}
		next.ServeHTTP(recorder, request)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		level := slog.LevelInfo
		if status >= http.StatusInternalServerError {
			level = slog.LevelError
		} else if status >= http.StatusBadRequest {
			level = slog.LevelWarn
		}
		logger.Log(request.Context(), level, "http request completed",
			"method", request.Method,
			"route", routeLabel(request.Method, request.URL.Path),
			"status", status,
			"duration_ms", time.Since(started).Milliseconds(),
		)
	})
}

func routeLabel(method, path string) string {
	switch method + " " + path {
	case "GET /healthz", "POST /v1/decisions", "POST /v1/checks", "GET /v1/tools", "POST /v1/tools/call", "POST /v1/chat", "GET /openapi.yaml", "GET /swagger":
		return path
	default:
		return "unmatched"
	}
}
