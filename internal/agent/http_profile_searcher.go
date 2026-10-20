package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const maxProfileSearchResponseBytes = 1 << 20

type HTTPProfileSearcher struct {
	endpoint string
	client   *http.Client
	apiKey   string
}

func NewHTTPProfileSearcher(endpoint string, client *http.Client, apiKey string) (*HTTPProfileSearcher, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, fmt.Errorf("profile search endpoint must be an absolute HTTPS URL")
	}
	if client == nil {
		client = &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
			},
		}
	}
	return &HTTPProfileSearcher{endpoint: endpoint, client: client, apiKey: apiKey}, nil
}

func (searcher *HTTPProfileSearcher) SearchPublicProfiles(ctx context.Context, lookup PersonLookup) ([]ProfilePage, error) {
	requestBody, err := json.Marshal(lookup)
	if err != nil {
		return nil, fmt.Errorf("encode profile search request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, searcher.endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("create profile search request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	if searcher.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+searcher.apiKey)
	}

	response, err := searcher.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call profile search provider: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("profile search provider returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxProfileSearchResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read profile search response: %w", err)
	}
	if len(body) > maxProfileSearchResponseBytes {
		return nil, fmt.Errorf("profile search response exceeds %d bytes", maxProfileSearchResponseBytes)
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var result struct {
		Candidates []struct {
			URL         string `json:"url"`
			Platform    string `json:"platform"`
			DisplayName string `json:"display_name"`
		} `json:"candidates"`
	}
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("decode profile search response: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("decode profile search response: unexpected trailing JSON data")
	}
	if len(result.Candidates) > maxCandidateProfiles {
		return nil, ErrTooManySearchResults
	}

	profiles := make([]ProfilePage, 0, len(result.Candidates))
	for _, candidate := range result.Candidates {
		profiles = append(profiles, ProfilePage{
			URL:         candidate.URL,
			Platform:    candidate.Platform,
			DisplayName: candidate.DisplayName,
		})
	}
	return profiles, nil
}
