package decision

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sdk "github.com/OpenRouterTeam/go-sdk"
	"github.com/OpenRouterTeam/go-sdk/models/components"
	"github.com/OpenRouterTeam/go-sdk/optionalnullable"

	"donna/internal/openrouter"
)

type OpenRouterClassifier struct {
	client openrouter.ChatCompleter
	model  string
}

func NewOpenRouterClassifier(client openrouter.ChatCompleter, model string) (*OpenRouterClassifier, error) {
	if client == nil {
		return nil, errors.New("OpenRouter client is required")
	}
	if strings.TrimSpace(model) == "" {
		return nil, errors.New("OpenRouter model is required")
	}
	return &OpenRouterClassifier{client: client, model: strings.TrimSpace(model)}, nil
}

func (classifier *OpenRouterClassifier) Classify(ctx context.Context, query string) (Scores, error) {
	responseFormat := components.CreateResponseFormatJSONObject(components.ChatFormatJSONObjectConfig{
		Type: components.ChatFormatJSONObjectConfigTypeJSONObject,
	})
	response, err := classifier.client.Complete(ctx, components.ChatRequest{
		Model: sdk.String(classifier.model),
		Messages: []components.ChatMessages{
			components.CreateChatMessagesSystem(components.ChatSystemMessage{
				Content: components.CreateChatSystemMessageContentStr("Classify the user's query for memory routing. Return only JSON with exactly five numeric probabilities: answer_directly, structured_lookup, semantic_search, hybrid_search, clarify. Each must be between 0 and 1 and all must sum to 1. Do not answer the query."),
			}),
			components.CreateChatMessagesUser(components.ChatUserMessage{
				Content: components.CreateChatUserMessageContentStr(query),
			}),
		},
		ResponseFormat:      &responseFormat,
		Temperature:         optionalnullable.From(sdk.Float64(0)),
		MaxCompletionTokens: optionalnullable.From(sdk.Int64(128)),
	})
	if err != nil {
		return Scores{}, fmt.Errorf("request OpenRouter route classification: %w", err)
	}
	content, err := assistantText(response)
	if err != nil {
		return Scores{}, err
	}
	return DecodeScores([]byte(content))
}

func (classifier *OpenRouterClassifier) Evaluate(ctx context.Context, question, evidence string) (YesNoScores, error) {
	responseFormat := components.CreateResponseFormatJSONObject(components.ChatFormatJSONObjectConfig{
		Type: components.ChatFormatJSONObjectConfigTypeJSONObject,
	})
	response, err := classifier.client.Complete(ctx, components.ChatRequest{
		Model: sdk.String(classifier.model),
		Messages: []components.ChatMessages{
			components.CreateChatMessagesSystem(components.ChatSystemMessage{
				Content: components.CreateChatSystemMessageContentStr("Evaluate the question using only the supplied evidence. Return only JSON with exactly two numeric probabilities: yes and no. Each must be between 0 and 1 and they must sum to 1. If evidence is insufficient, return equal probabilities."),
			}),
			components.CreateChatMessagesUser(components.ChatUserMessage{
				Content: components.CreateChatUserMessageContentStr(fmt.Sprintf("Question: %s\nEvidence: %s", question, evidence)),
			}),
		},
		ResponseFormat:      &responseFormat,
		Temperature:         optionalnullable.From(sdk.Float64(0)),
		MaxCompletionTokens: optionalnullable.From(sdk.Int64(64)),
	})
	if err != nil {
		return YesNoScores{}, fmt.Errorf("request OpenRouter yes/no evaluation: %w", err)
	}
	content, err := assistantText(response)
	if err != nil {
		return YesNoScores{}, err
	}
	return DecodeYesNoScores([]byte(content))
}

func assistantText(response components.ChatResult) (string, error) {
	if len(response.Choices) == 0 {
		return "", errors.New("OpenRouter response contains no choices")
	}
	content, ok := response.Choices[0].Message.Content.GetOrZero()
	if !ok || content.Str == nil {
		return "", errors.New("OpenRouter response has no text content")
	}
	return strings.TrimSpace(*content.Str), nil
}
