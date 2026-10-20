package agent

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"donna/internal/openrouter"
)

type ChatRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id,omitempty"`
}

type ChatResponse struct {
	Answer    string `json:"answer"`
	SessionID string `json:"session_id"`
}

type RuntimeConfig struct {
	MaxModelTurns      int
	MaxToolCalls       int
	SystemPrompt       string
	MaxSessions        int
	SessionTTL         time.Duration
	MaxHistoryMessages int
}

type Runtime struct {
	brain              *Brain
	model              openrouter.ChatCompleter
	modelName          string
	maxModelTurns      int
	maxToolCalls       int
	systemPrompt       string
	sessionsMu         sync.Mutex
	sessions           map[string]*chatSession
	maxSessions        int
	sessionTTL         time.Duration
	maxHistoryMessages int
}

func NewRuntime(brain *Brain, model openrouter.ChatCompleter, modelName string, config RuntimeConfig) (*Runtime, error) {
	if brain == nil || model == nil || strings.TrimSpace(modelName) == "" {
		return nil, errors.New("brain, OpenRouter client, and model name are required")
	}
	if config.MaxModelTurns == 0 {
		config.MaxModelTurns = defaultMaxModelTurns
	}
	if config.MaxToolCalls == 0 {
		config.MaxToolCalls = defaultMaxToolCalls
	}
	if config.MaxSessions == 0 {
		config.MaxSessions = defaultMaxSessions
	}
	if config.SessionTTL == 0 {
		config.SessionTTL = defaultSessionTTL
	}
	if config.MaxHistoryMessages == 0 {
		config.MaxHistoryMessages = defaultMaxHistoryMessages
	}
	if config.MaxModelTurns < 1 || config.MaxToolCalls < 1 {
		return nil, errors.New("agent turn and tool-call limits must be positive")
	}
	if config.MaxSessions < 1 || config.SessionTTL < time.Minute || config.MaxHistoryMessages < 1 {
		return nil, errors.New("agent session limits and TTL must be positive")
	}
	systemPrompt := strings.TrimSpace(config.SystemPrompt)
	if systemPrompt == "" {
		systemPrompt = strings.TrimSpace(defaultSystemPrompt)
	}
	if systemPrompt == "" || len(systemPrompt) > maxSystemPromptBytes {
		return nil, fmt.Errorf("system prompt must be non-empty and at most %d bytes", maxSystemPromptBytes)
	}
	return &Runtime{
		brain: brain, model: model, modelName: strings.TrimSpace(modelName),
		maxModelTurns: config.MaxModelTurns, maxToolCalls: config.MaxToolCalls,
		systemPrompt: systemPrompt, sessions: make(map[string]*chatSession),
		maxSessions: config.MaxSessions, sessionTTL: config.SessionTTL,
		maxHistoryMessages: config.MaxHistoryMessages,
	}, nil
}

func LoadSystemPrompt(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return strings.TrimSpace(defaultSystemPrompt), nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read system prompt file: %w", err)
	}
	prompt := strings.TrimSpace(string(content))
	if prompt == "" || len(prompt) > maxSystemPromptBytes {
		return "", fmt.Errorf("system prompt file must be non-empty and at most %d bytes", maxSystemPromptBytes)
	}
	return prompt, nil
}
