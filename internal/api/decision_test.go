package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"donna/internal/agent"
	"donna/internal/decision"
)

type testDecisionService struct {
	result      decision.Result
	err         error
	query       string
	checkResult decision.CheckResult
	checkErr    error
	question    string
	evidence    string
}

func (service *testDecisionService) Decide(_ context.Context, query string) (decision.Result, error) {
	service.query = query
	return service.result, service.err
}

func (service *testDecisionService) Check(_ context.Context, question, evidence string) (decision.CheckResult, error) {
	service.question = question
	service.evidence = evidence
	return service.checkResult, service.checkErr
}

type testToolService struct {
	name      string
	arguments json.RawMessage
}

type testChatService struct {
	request agent.ChatRequest
	result  agent.ChatResponse
	err     error
}

func (service *testChatService) Reply(_ context.Context, request agent.ChatRequest) (agent.ChatResponse, error) {
	service.request = request
	return service.result, service.err
}

func (service *testToolService) Tools() []agent.ToolDefinition {
	return []agent.ToolDefinition{{Name: agent.ToolCheckYesNo, Description: "Check evidence", InputSchema: json.RawMessage(`{"type":"object"}`)}}
}

func (service *testToolService) InvokeTool(_ context.Context, name string, arguments json.RawMessage) (json.RawMessage, error) {
	service.name = name
	service.arguments = arguments
	return json.RawMessage(`{"answer":null,"confidence":0.5}`), nil
}

func TestDecisionEndpointReturnsRoute(t *testing.T) {
	service := &testDecisionService{result: decision.Result{Action: decision.SemanticSearch, Confidence: 0.8}}
	handler := NewHandler(service)
	request := httptest.NewRequest(http.MethodPost, "/v1/decisions", strings.NewReader(`{"query":"find my note"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.Code)
	}
	if service.query != "find my note" || !strings.Contains(response.Body.String(), `"action":"semantic_search"`) {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
}

func TestDecisionEndpointRejectsMalformedRequests(t *testing.T) {
	for _, payload := range []string{`{`, `{"query":"ok","other":true}`, `{"query":"ok"} {}`} {
		handler := NewHandler(&testDecisionService{})
		request := httptest.NewRequest(http.MethodPost, "/v1/decisions", strings.NewReader(payload))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Errorf("payload %q: expected 400, got %d", payload, response.Code)
		}
	}
}

func TestDecisionEndpointMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "empty query", err: decision.ErrEmptyQuery, want: http.StatusBadRequest},
		{name: "provider failure", err: errors.New("provider unavailable"), want: http.StatusBadGateway},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewHandler(&testDecisionService{err: test.err})
			request := httptest.NewRequest(http.MethodPost, "/v1/decisions", strings.NewReader(`{"query":"hello"}`))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("expected %d, got %d", test.want, response.Code)
			}
		})
	}
}

func TestCheckEndpointReturnsYesNoResult(t *testing.T) {
	answer := true
	service := &testDecisionService{checkResult: decision.CheckResult{Answer: &answer, Confidence: 0.9}}
	handler := NewHandler(service)
	request := httptest.NewRequest(http.MethodPost, "/v1/checks", strings.NewReader(`{"question":"Is it saved?","evidence":"project notes"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || service.question != "Is it saved?" || service.evidence != "project notes" {
		t.Fatalf("unexpected check response: status=%d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"answer":true`) {
		t.Fatalf("expected yes answer, got %s", response.Body.String())
	}
}

func TestCheckEndpointReturnsUncertainAsNull(t *testing.T) {
	service := &testDecisionService{checkResult: decision.CheckResult{Confidence: 0.5}}
	handler := NewHandler(service)
	request := httptest.NewRequest(http.MethodPost, "/v1/checks", strings.NewReader(`{"question":"Is it saved?"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"answer":null`) {
		t.Fatalf("expected uncertain response, got status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestToolEndpointsRequireBearerTokenAndDispatch(t *testing.T) {
	toolService := &testToolService{}
	handler := NewHandler(&testDecisionService{}, HandlerOptions{ToolService: toolService, ToolToken: "agent-secret"})

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/v1/tools", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing token, got %d", unauthorized.Code)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/v1/tools", nil)
	listRequest.Header.Set("Authorization", "Bearer agent-secret")
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), agent.ToolCheckYesNo) {
		t.Fatalf("unexpected tool list response: status=%d body=%s", listResponse.Code, listResponse.Body.String())
	}

	callRequest := httptest.NewRequest(http.MethodPost, "/v1/tools/call", strings.NewReader(`{"name":"donna_check_yes_no","arguments":{"question":"Is it saved?","evidence":"notes"}}`))
	callRequest.Header.Set("Authorization", "Bearer agent-secret")
	callResponse := httptest.NewRecorder()
	handler.ServeHTTP(callResponse, callRequest)
	if callResponse.Code != http.StatusOK || toolService.name != agent.ToolCheckYesNo || !strings.Contains(callResponse.Body.String(), `"answer":null`) {
		t.Fatalf("unexpected tool call response: status=%d body=%s", callResponse.Code, callResponse.Body.String())
	}
}

func TestToolCallRejectsMalformedPayload(t *testing.T) {
	handler := NewHandler(&testDecisionService{}, HandlerOptions{ToolService: &testToolService{}, ToolToken: "agent-secret"})
	request := httptest.NewRequest(http.MethodPost, "/v1/tools/call", strings.NewReader(`{"name":"donna_check_yes_no","arguments":{}} {}`))
	request.Header.Set("Authorization", "Bearer agent-secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed tool call, got %d", response.Code)
	}
}

func TestChatEndpointRequiresBearerAndReturnsAnswer(t *testing.T) {
	chatService := &testChatService{result: agent.ChatResponse{Answer: "Hello there.", SessionID: "session-123"}}
	handler := NewHandler(&testDecisionService{}, HandlerOptions{ChatService: chatService, ToolToken: "chat-secret"})
	request := httptest.NewRequest(http.MethodPost, "/v1/chat", strings.NewReader(`{"message":"hello"}`))
	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, request)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without bearer token, got %d", unauthorized.Code)
	}
	rawTokenRequest := httptest.NewRequest(http.MethodPost, "/v1/chat", strings.NewReader(`{"message":"hello"}`))
	rawTokenRequest.Header.Set("Authorization", "chat-secret")
	rawTokenResponse := httptest.NewRecorder()
	handler.ServeHTTP(rawTokenResponse, rawTokenRequest)
	if rawTokenResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without Bearer scheme, got %d", rawTokenResponse.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/v1/chat", strings.NewReader(`{"message":"hello"}`))
	request.Header.Set("Authorization", "Bearer chat-secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || chatService.request.Message != "hello" || !strings.Contains(response.Body.String(), `"answer":"Hello there."`) || !strings.Contains(response.Body.String(), `"session_id":"session-123"`) {
		t.Fatalf("unexpected chat response: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestChatEndpointPassesSessionID(t *testing.T) {
	chatService := &testChatService{result: agent.ChatResponse{Answer: "Follow-up answer.", SessionID: "session-123"}}
	handler := NewHandler(&testDecisionService{}, HandlerOptions{ChatService: chatService, ToolToken: "chat-secret"})
	request := httptest.NewRequest(http.MethodPost, "/v1/chat", strings.NewReader(`{"message":"dig deeper","session_id":"session-123"}`))
	request.Header.Set("Authorization", "Bearer chat-secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || chatService.request.SessionID != "session-123" {
		t.Fatalf("session ID was not passed through: status=%d request=%+v", response.Code, chatService.request)
	}
}

func TestOpenAPISpecAndSwaggerUIAreServed(t *testing.T) {
	handler := NewHandler(&testDecisionService{})
	for _, path := range []string{"/openapi.yaml", "/swagger"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK || response.Body.Len() == 0 {
			t.Errorf("%s: expected non-empty 200, got status=%d", path, response.Code)
		}
	}
}
