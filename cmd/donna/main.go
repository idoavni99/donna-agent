package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"donna/internal/agent"
	"donna/internal/api"
	"donna/internal/decision"
	"donna/internal/openrouter"

	"github.com/joho/godotenv"
)

func main() {
	if err := loadEnvFile(".env"); err != nil {
		log.Fatalf("load .env: %v", err)
	}
	level, err := parseLogLevel(os.Getenv("DONNA_LOG_LEVEL"))
	if err != nil {
		log.Fatal(err)
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
	slog.Info("Donna startup", "log_level", level.String(), "dotenv_file_present", fileExists(".env"))

	openRouterKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	if openRouterKey == "" {
		log.Fatal("OPENROUTER_API_KEY is required")
	}
	modelName := strings.TrimSpace(os.Getenv("OPENROUTER_MODEL"))
	if modelName == "" {
		log.Fatal("OPENROUTER_MODEL is required")
	}
	agentToken := strings.TrimSpace(os.Getenv("DONNA_AGENT_API_KEY"))
	if agentToken == "" || strings.EqualFold(agentToken, "replace-with-a-local-token") || strings.EqualFold(agentToken, "local-agent-token") {
		log.Fatal("DONNA_AGENT_API_KEY must be set to a non-placeholder token to protect model-backed routes")
	}

	minimumConfidence := 0.55
	if value := strings.TrimSpace(os.Getenv("DONNA_MIN_CONFIDENCE")); value != "" {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			log.Fatalf("invalid DONNA_MIN_CONFIDENCE: %v", err)
		}
		minimumConfidence = parsed
	}

	openRouterClient, err := openrouter.NewClient(openrouter.Config{
		APIKey:  openRouterKey,
		Model:   modelName,
		AppName: "Donna",
		AppURL:  os.Getenv("OPENROUTER_APP_URL"),
	})
	if err != nil {
		log.Fatalf("configure OpenRouter: %v", err)
	}
	classifier, err := decision.NewOpenRouterClassifier(openRouterClient, modelName)
	if err != nil {
		log.Fatalf("configure OpenRouter decision classifier: %v", err)
	}
	engine, err := decision.NewEngineWithCheck(classifier, classifier, minimumConfidence)
	if err != nil {
		log.Fatalf("configure decision engine: %v", err)
	}
	var profileSearcher agent.PublicProfileSearcher
	if searchEndpoint := strings.TrimSpace(os.Getenv("DONNA_PROFILE_SEARCH_URL")); searchEndpoint != "" {
		profileSearcher, err = agent.NewHTTPProfileSearcher(searchEndpoint, nil, os.Getenv("DONNA_PROFILE_SEARCH_API_KEY"))
		if err != nil {
			log.Fatalf("configure public profile search: %v", err)
		}
	}
	brain, err := agent.NewBrainWithProfileSearch(nil, nil, profileSearcher, engine)
	if err != nil {
		log.Fatalf("configure agent brain: %v", err)
	}
	systemPrompt, err := agent.LoadSystemPrompt(os.Getenv("DONNA_SYSTEM_PROMPT_FILE"))
	if err != nil {
		log.Fatalf("configure system prompt: %v", err)
	}
	runtime, err := agent.NewRuntime(brain, openRouterClient, modelName, agent.RuntimeConfig{SystemPrompt: systemPrompt})
	if err != nil {
		log.Fatalf("configure agent runtime: %v", err)
	}
	slog.Info("Donna configured", "model", modelName, "profile_search_enabled", profileSearcher != nil, "agent_api_enabled", strings.TrimSpace(os.Getenv("DONNA_AGENT_API_KEY")) != "", "external_system_prompt", strings.TrimSpace(os.Getenv("DONNA_SYSTEM_PROMPT_FILE")) != "")

	address := strings.TrimSpace(os.Getenv("DONNA_ADDR"))
	if address == "" {
		address = "127.0.0.1:8080"
	}
	server := &http.Server{
		Addr: address,
		Handler: api.NewHandler(brain, api.HandlerOptions{
			ToolService: brain,
			ChatService: runtime,
			ToolToken:   agentToken,
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			slog.Error("HTTP server shutdown failed", "error_type", fmt.Sprintf("%T", err))
		}
	}()

	slog.Info("HTTP server listening", "address", address)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve HTTP: %v", err)
	} else {
		slog.Info("HTTP server stopped")
	}
}

func loadEnvFile(path string) error {
	if err := godotenv.Load(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func parseLogLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("DONNA_LOG_LEVEL must be debug, info, warn, or error")
	}
}
