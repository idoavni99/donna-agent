package openrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OpenRouterTeam/go-sdk/models/components"
)

func TestClientUsesSDKForChatCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.HasSuffix(request.URL.Path, "/chat/completions") || request.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected SDK request: path=%q authorization=%q", request.URL.Path, request.Header.Get("Authorization"))
		}
		var payload components.ChatRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Model == nil || *payload.Model != "test/model" {
			t.Errorf("unexpected model: %+v", payload.Model)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"test","object":"chat.completion","created":1,"model":"test/model","choices":[{"index":0,"message":{"role":"assistant","content":"hello from SDK"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{APIKey: "test-key", Model: "test/model", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Complete(context.Background(), components.ChatRequest{
		Messages: []components.ChatMessages{
			components.CreateChatMessagesUser(components.ChatUserMessage{Content: components.CreateChatUserMessageContentStr("hello")}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Choices) != 1 {
		t.Fatalf("unexpected SDK response: %+v", response)
	}
	content, ok := response.Choices[0].Message.Content.GetOrZero()
	if !ok || content.Str == nil || *content.Str != "hello from SDK" {
		t.Fatalf("unexpected message content: %+v", content)
	}
}

func TestNewClientValidatesRequiredConfig(t *testing.T) {
	for _, config := range []Config{
		{Model: "test/model"},
		{APIKey: "key"},
		{APIKey: "key", Model: "test/model", BaseURL: "http://example.com"},
		{APIKey: "key", Model: "test/model", BaseURL: "http://127.attacker.example"},
	} {
		if _, err := NewClient(config); err == nil {
			t.Errorf("expected config to fail validation: %+v", config)
		}
	}
}
