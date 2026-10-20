package api

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLoggingRecordsRouteWithoutQueryOrBody(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	handler := logRequests(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusAccepted)
	}), logger)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat?private=query-value", strings.NewReader("private body-value"))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	logLine := output.String()
	if response.Code != http.StatusAccepted || !strings.Contains(logLine, `route=/v1/chat`) || !strings.Contains(logLine, `status=202`) || !strings.Contains(logLine, `duration_ms=`) {
		t.Fatalf("missing request fields: %s", logLine)
	}
	if strings.Contains(logLine, "private") || strings.Contains(logLine, "query-value") || strings.Contains(logLine, "body-value") {
		t.Fatalf("request log leaked input: %s", logLine)
	}
}
