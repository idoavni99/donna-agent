package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenRouterTeam/go-sdk/models/components"
	"github.com/OpenRouterTeam/go-sdk/optionalnullable"

	"donna/internal/decision"
	"donna/internal/openrouter"
)

type testChatCompleter struct {
	responses []components.ChatResult
	requests  []components.ChatRequest
}

func (model *testChatCompleter) Complete(_ context.Context, request components.ChatRequest) (components.ChatResult, error) {
	model.requests = append(model.requests, request)
	if len(model.responses) == 0 {
		return components.ChatResult{}, errors.New("no test response configured")
	}
	response := model.responses[0]
	model.responses = model.responses[1:]
	return response, nil
}

func TestRuntimeRunsToolCallAndReturnsFinalAnswer(t *testing.T) {
	answer := true
	layer := &toolTestDecisionLayer{checkResult: decision.CheckResult{Answer: &answer, Confidence: 0.9}}
	brain, err := NewBrain(nil, nil, layer)
	if err != nil {
		t.Fatal(err)
	}
	toolMessage := components.ChatAssistantMessage{ToolCalls: []components.ChatToolCall{{
		ID:   "call-1",
		Type: components.ChatToolCallTypeFunction,
		Function: components.ChatToolCallFunction{
			Name:      ToolCheckYesNo,
			Arguments: `{"question":"Is it saved?","evidence":"Yes, in project notes"}`,
		},
	}}}
	model := &testChatCompleter{responses: []components.ChatResult{
		{Choices: []components.ChatChoice{{Message: toolMessage}}},
		{Choices: []components.ChatChoice{{Message: assistantTextMessage("Yes, it is saved.")}}},
	}}
	runtime, err := NewRuntime(brain, model, "test/model", RuntimeConfig{})
	if err != nil {
		t.Fatal(err)
	}

	response, err := runtime.Reply(context.Background(), ChatRequest{Message: "Is it saved?"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Answer != "Yes, it is saved." || response.SessionID == "" || layer.question != "Is it saved?" || len(model.requests) != 2 {
		t.Fatalf("unexpected runtime result: response=%+v question=%q turns=%d", response, layer.question, len(model.requests))
	}
	if len(model.requests[1].Messages) != 4 || model.requests[1].Messages[3].ChatToolMessage.ToolCallID != "call-1" {
		t.Fatalf("tool result was not replayed to the model: %+v", model.requests[1].Messages)
	}
	if len(model.requests[0].Tools) != 3 || model.requests[0].Model == nil || *model.requests[0].Model != "test/model" {
		t.Fatalf("expected configured model and brain tools: %+v", model.requests[0])
	}
	if model.requests[0].Tools[2].OpenRouterWebSearchServerTool == nil {
		t.Fatal("native web search should be available without a separate search API key")
	}
}

func TestRuntimeOffersNativeWebSearchWithThreeResultCap(t *testing.T) {
	brain, err := NewBrain(nil, nil, &toolTestDecisionLayer{})
	if err != nil {
		t.Fatal(err)
	}
	model := &testChatCompleter{responses: []components.ChatResult{{Choices: []components.ChatChoice{{Message: assistantTextMessage("I found relevant sources.")}}}}}
	runtime, err := NewRuntime(brain, model, "test/model", RuntimeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Reply(context.Background(), ChatRequest{Message: "Search public sources about Alex Lee's design work"}); err != nil {
		t.Fatal(err)
	}
	if len(model.requests[0].Tools) != 3 || model.requests[0].Tools[2].OpenRouterWebSearchServerTool == nil {
		t.Fatalf("expected built-in search without an external search key: %+v", model.requests[0].Tools)
	}
	config := model.requests[0].Tools[2].OpenRouterWebSearchServerTool.Parameters
	if config == nil || config.Engine == nil || *config.Engine != components.WebSearchEngineEnumNative || config.MaxResults == nil || *config.MaxResults != 3 || config.MaxTotalResults == nil || *config.MaxTotalResults != 3 {
		t.Fatalf("expected native web search capped at three results: %+v", config)
	}
}

func TestRuntimeRejectsEmptyMessageAndBoundsToolCalls(t *testing.T) {
	brain, err := NewBrain(nil, nil, &toolTestDecisionLayer{})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewRuntime(brain, &testChatCompleter{}, "test/model", RuntimeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Reply(context.Background(), ChatRequest{Message: "  "}); !errors.Is(err, ErrEmptyChatMessage) {
		t.Fatalf("expected ErrEmptyChatMessage, got %v", err)
	}
	if _, err := NewRuntime(brain, &testChatCompleter{}, "test/model", RuntimeConfig{MaxModelTurns: -1}); err == nil {
		t.Fatal("expected invalid model-turn limit to be rejected")
	}
}

func TestRuntimeUsesConfiguredSystemPrompt(t *testing.T) {
	brain, err := NewBrain(nil, nil, &toolTestDecisionLayer{})
	if err != nil {
		t.Fatal(err)
	}
	model := &testChatCompleter{responses: []components.ChatResult{{Choices: []components.ChatChoice{{Message: assistantTextMessage("Ready.")}}}}}
	runtime, err := NewRuntime(brain, model, "test/model", RuntimeConfig{SystemPrompt: "External Donna prompt."})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Reply(context.Background(), ChatRequest{Message: "Hello"}); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(model.requests[0].Messages[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), "External Donna prompt.") {
		t.Fatalf("configured prompt missing from model request: %s", encoded)
	}
}

func TestRuntimeEnablesRestrictedWebFetchForPublicURL(t *testing.T) {
	brain, err := NewBrain(nil, nil, &toolTestDecisionLayer{})
	if err != nil {
		t.Fatal(err)
	}
	model := &testChatCompleter{responses: []components.ChatResult{{Choices: []components.ChatChoice{{Message: assistantTextMessage("This page explains the topic.")}}}}}
	runtime, err := NewRuntime(brain, model, "test/model", RuntimeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Reply(context.Background(), ChatRequest{Message: "Summarize https://example.com/article"}); err != nil {
		t.Fatal(err)
	}
	tools := model.requests[0].Tools
	if len(tools) != 4 || tools[2].OpenRouterWebSearchServerTool == nil || tools[3].WebFetchServerTool == nil {
		t.Fatalf("expected conditional web-fetch tool: %+v", tools)
	}
	config := tools[3].WebFetchServerTool.Parameters
	if config == nil || len(config.AllowedDomains) != 1 || config.AllowedDomains[0] != "example.com" || config.MaxUses == nil || *config.MaxUses != maxFetchURLsPerMessage {
		t.Fatalf("web fetch was not restricted to supplied domain: %+v", config)
	}
}

func TestRuntimeContinuesSessionAndSelectedURL(t *testing.T) {
	brain, err := NewBrain(nil, nil, &toolTestDecisionLayer{})
	if err != nil {
		t.Fatal(err)
	}
	model := &testChatCompleter{responses: []components.ChatResult{
		{Choices: []components.ChatChoice{{Message: assistantTextMessage("The page introduces Alex as a designer.")}}},
		{Choices: []components.ChatChoice{{Message: assistantTextMessage("The same page also lists a published design project.")}}},
	}}
	runtime, err := NewRuntime(brain, model, "test/model", RuntimeConfig{})
	if err != nil {
		t.Fatal(err)
	}

	first, err := runtime.Reply(context.Background(), ChatRequest{Message: "Research this person: https://example.com/alex"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := runtime.Reply(context.Background(), ChatRequest{Message: "Dig deeper into their work", SessionID: first.SessionID})
	if err != nil {
		t.Fatal(err)
	}
	if second.SessionID != first.SessionID || len(model.requests) != 2 {
		t.Fatalf("follow-up did not reuse session: first=%+v second=%+v turns=%d", first, second, len(model.requests))
	}
	request := model.requests[1]
	if len(request.Messages) != 4 || request.Messages[1].ChatUserMessage == nil || request.Messages[2].ChatAssistantMessage == nil {
		t.Fatalf("prior exchange missing from follow-up: %+v", request.Messages)
	}
	if len(request.Tools) != 4 || request.Tools[2].OpenRouterWebSearchServerTool == nil || request.Tools[3].WebFetchServerTool == nil || request.Tools[3].WebFetchServerTool.Parameters.AllowedDomains[0] != "example.com" {
		t.Fatalf("follow-up did not retain approved URL host: %+v", request.Tools)
	}
	systemContent := request.Messages[0].ChatSystemMessage.Content.Str
	if systemContent == nil || !strings.Contains(*systemContent, "https://example.com/alex") {
		t.Fatal("selected URL was not included in follow-up context")
	}
}

func TestRuntimeRejectsUnknownOrInvalidSessionIDs(t *testing.T) {
	brain, err := NewBrain(nil, nil, &toolTestDecisionLayer{})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewRuntime(brain, &testChatCompleter{}, "test/model", RuntimeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := runtime.acquireSession("session id with spaces"); !errors.Is(err, ErrInvalidChatSessionID) {
		t.Fatalf("expected invalid session ID error, got %v", err)
	}
	if _, _, err := runtime.acquireSession("unknown-session"); !errors.Is(err, ErrChatSessionNotFound) {
		t.Fatalf("expected unknown session error, got %v", err)
	}
}

func TestChatSessionHistoryCapKeepsCompleteExchanges(t *testing.T) {
	session := &chatSession{}
	for _, text := range []string{"one", "two", "three"} {
		user := components.CreateChatMessagesUser(components.ChatUserMessage{Content: components.CreateChatUserMessageContentStr(text)})
		assistant := components.CreateChatMessagesAssistant(assistantTextMessage(text + " reply"))
		session.appendExchange([]components.ChatMessages{user, assistant}, 3)
	}
	if session.historyCount > 3 || session.historyCount != 2 || len(session.exchanges) != 1 {
		t.Fatalf("history did not retain only complete recent exchanges: count=%d exchanges=%d", session.historyCount, len(session.exchanges))
	}
}

func TestLoadSystemPromptDefaultAndFileOverride(t *testing.T) {
	defaultPrompt, err := LoadSystemPrompt("")
	if err != nil || defaultPrompt == "" {
		t.Fatalf("expected embedded default prompt, got %q, %v", defaultPrompt, err)
	}
	path := filepath.Join(t.TempDir(), "custom-prompt.txt")
	if err := os.WriteFile(path, []byte("  Custom prompt from file.  \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	customPrompt, err := LoadSystemPrompt(path)
	if err != nil {
		t.Fatal(err)
	}
	if customPrompt != "Custom prompt from file." {
		t.Fatalf("unexpected external prompt: %q", customPrompt)
	}
}

func TestLoadSystemPromptRejectsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty-prompt.txt")
	if err := os.WriteFile(path, []byte(" \n "), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSystemPrompt(path); err == nil {
		t.Fatal("expected empty prompt file to be rejected")
	}
}

func assistantTextMessage(text string) components.ChatAssistantMessage {
	content := components.CreateChatAssistantMessageContentStr(text)
	return components.ChatAssistantMessage{Content: optionalnullable.From(&content)}
}

var _ openrouter.ChatCompleter = (*testChatCompleter)(nil)
