package decision

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const maxDecisionResponseBytes = 1 << 20

type HTTPClassifier struct {
	endpoint string
	client   *http.Client
	apiKey   string
}

func NewHTTPClassifier(endpoint string, client *http.Client, apiKey string) (*HTTPClassifier, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("classifier endpoint must be an absolute HTTP or HTTPS URL")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &HTTPClassifier{endpoint: endpoint, client: client, apiKey: apiKey}, nil
}

func (classifier *HTTPClassifier) Classify(ctx context.Context, query string) (Scores, error) {
	requestBody, err := json.Marshal(struct {
		Query string `json:"query"`
	}{Query: query})
	if err != nil {
		return Scores{}, fmt.Errorf("encode classifier request: %w", err)
	}
	body, err := classifier.postJSON(ctx, requestBody)
	if err != nil {
		return Scores{}, err
	}
	scores, err := DecodeScores(body)
	if err != nil {
		return Scores{}, fmt.Errorf("decode classifier response: %w", err)
	}
	return scores, nil
}

func (classifier *HTTPClassifier) Evaluate(ctx context.Context, question, evidence string) (YesNoScores, error) {
	requestBody, err := json.Marshal(struct {
		Question string `json:"question"`
		Evidence string `json:"evidence"`
	}{Question: question, Evidence: evidence})
	if err != nil {
		return YesNoScores{}, fmt.Errorf("encode yes/no request: %w", err)
	}
	body, err := classifier.postJSON(ctx, requestBody)
	if err != nil {
		return YesNoScores{}, err
	}
	scores, err := DecodeYesNoScores(body)
	if err != nil {
		return YesNoScores{}, fmt.Errorf("decode yes/no response: %w", err)
	}
	return scores, nil
}

func (classifier *HTTPClassifier) postJSON(ctx context.Context, body []byte) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, classifier.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create classifier request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	if classifier.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+classifier.apiKey)
	}

	response, err := classifier.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call decision classifier: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("decision classifier returned HTTP %d", response.StatusCode)
	}

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxDecisionResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read classifier response: %w", err)
	}
	if len(responseBody) > maxDecisionResponseBytes {
		return nil, fmt.Errorf("classifier response exceeds %d bytes", maxDecisionResponseBytes)
	}
	return responseBody, nil
}
