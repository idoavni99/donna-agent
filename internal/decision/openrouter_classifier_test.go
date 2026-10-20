package decision

import (
	"context"
	"testing"

	"github.com/OpenRouterTeam/go-sdk/models/components"
	"github.com/OpenRouterTeam/go-sdk/optionalnullable"

	"donna/internal/openrouter"
)

type testOpenRouterClient struct {
	responses []components.ChatResult
	requests  []components.ChatRequest
}

func (client *testOpenRouterClient) Complete(_ context.Context, request components.ChatRequest) (components.ChatResult, error) {
	client.requests = append(client.requests, request)
	response := client.responses[0]
	client.responses = client.responses[1:]
	return response, nil
}

func TestOpenRouterClassifierUsesJSONModeAndValidatedScores(t *testing.T) {
	client := &testOpenRouterClient{responses: []components.ChatResult{
		assistantJSON(`{"answer_directly":0.05,"structured_lookup":0.1,"semantic_search":0.7,"hybrid_search":0.1,"clarify":0.05}`),
		assistantJSON(`{"yes":0.9,"no":0.1}`),
	}}
	classifier, err := NewOpenRouterClassifier(client, "openai/gpt-4o-mini")
	if err != nil {
		t.Fatal(err)
	}
	routeScores, err := classifier.Classify(context.Background(), "find my note")
	if err != nil {
		t.Fatal(err)
	}
	yesNoScores, err := classifier.Evaluate(context.Background(), "is it saved?", "yes, in project notes")
	if err != nil {
		t.Fatal(err)
	}
	if routeScores.SemanticSearch != 0.7 || yesNoScores.Yes != 0.9 || len(client.requests) != 2 {
		t.Fatalf("unexpected scores or requests: route=%+v yesNo=%+v", routeScores, yesNoScores)
	}
	for _, request := range client.requests {
		if request.Model == nil || *request.Model != "openai/gpt-4o-mini" || request.ResponseFormat == nil {
			t.Errorf("expected model and JSON response mode: %+v", request)
		}
	}
}

func assistantJSON(text string) components.ChatResult {
	content := components.CreateChatAssistantMessageContentStr(text)
	message := components.ChatAssistantMessage{Content: optionalnullable.From(&content)}
	return components.ChatResult{Choices: []components.ChatChoice{{Message: message}}}
}

var _ openrouter.ChatCompleter = (*testOpenRouterClient)(nil)
