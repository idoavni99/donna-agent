package openrouter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"time"

	sdk "github.com/OpenRouterTeam/go-sdk"
	"github.com/OpenRouterTeam/go-sdk/models/components"
)

type Config struct {
	APIKey  string
	Model   string
	BaseURL string
	AppName string
	AppURL  string
	Timeout time.Duration
}

type ChatCompleter interface {
	Complete(context.Context, components.ChatRequest) (components.ChatResult, error)
}

type Client struct {
	sdk   *sdk.OpenRouter
	model string
}

func NewClient(config Config) (*Client, error) {
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, errors.New("OpenRouter API key is required")
	}
	if strings.TrimSpace(config.Model) == "" {
		return nil, errors.New("OpenRouter model is required")
	}
	if config.Timeout <= 0 {
		config.Timeout = 90 * time.Second
	}
	options := []sdk.SDKOption{
		sdk.WithSecurity(strings.TrimSpace(config.APIKey)),
		sdk.WithTimeout(config.Timeout),
	}
	if baseURL := strings.TrimSpace(config.BaseURL); baseURL != "" {
		parsed, err := url.Parse(baseURL)
		if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname()))) {
			return nil, errors.New("OpenRouter base URL must use HTTPS (HTTP is allowed only for loopback tests)")
		}
		options = append(options, sdk.WithServerURL(strings.TrimRight(baseURL, "/")))
	}
	if appName := strings.TrimSpace(config.AppName); appName != "" {
		options = append(options, sdk.WithXTitle(appName))
	}
	if appURL := strings.TrimSpace(config.AppURL); appURL != "" {
		options = append(options, sdk.WithHTTPReferer(appURL))
	}
	return &Client{sdk: sdk.New(options...), model: strings.TrimSpace(config.Model)}, nil
}

func (client *Client) Complete(ctx context.Context, request components.ChatRequest) (components.ChatResult, error) {
	if len(request.Messages) == 0 {
		return components.ChatResult{}, errors.New("OpenRouter request requires at least one message")
	}
	if request.Model == nil {
		request.Model = sdk.String(client.model)
	}
	started := time.Now()
	toolCount := len(request.Tools)
	slog.DebugContext(ctx, "OpenRouter request started", "model", client.model, "message_count", len(request.Messages), "tool_count", toolCount)
	response, err := client.sdk.Chat.Send(ctx, request, nil)
	if err != nil {
		slog.WarnContext(ctx, "OpenRouter request failed", "model", client.model, "error_type", fmt.Sprintf("%T", err), "duration_ms", time.Since(started).Milliseconds())
		return components.ChatResult{}, fmt.Errorf("OpenRouter chat completion: %w", err)
	}
	if response == nil || response.ChatResult == nil {
		slog.WarnContext(ctx, "OpenRouter returned unexpected response type", "model", client.model, "duration_ms", time.Since(started).Milliseconds())
		return components.ChatResult{}, errors.New("OpenRouter returned an unexpected streaming response")
	}
	responseToolCalls := 0
	for _, choice := range response.ChatResult.Choices {
		responseToolCalls += len(choice.Message.ToolCalls)
	}
	slog.InfoContext(ctx, "OpenRouter request completed", "model", client.model, "choice_count", len(response.ChatResult.Choices), "tool_call_count", responseToolCalls, "duration_ms", time.Since(started).Milliseconds())
	return *response.ChatResult, nil
}

func (client *Client) Model() string {
	return client.model
}

var _ ChatCompleter = (*Client)(nil)

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
