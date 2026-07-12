package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// topicParser extracts candidate topic noun phrases from free-form text.
type topicParser interface {
	ParseTopics(ctx context.Context, text string) ([]string, error)
}

// NLPServiceClient calls POST /parse on the Python spaCy sidecar.
type NLPServiceClient struct {
	url    string
	client *http.Client
}

// NewNLPServiceClient constructs a client targeting the given base URL (e.g. "http://localhost:8090").
func NewNLPServiceClient(baseURL string) *NLPServiceClient {
	return &NLPServiceClient{
		url: baseURL + "/parse",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ParseTopics sends text to the NLP sidecar and returns the extracted noun phrases.
func (c *NLPServiceClient) ParseTopics(ctx context.Context, text string) ([]string, error) {
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return nil, fmt.Errorf("marshal parse request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build parse request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call NLP service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NLP service returned %d", resp.StatusCode)
	}

	var result struct {
		NounPhrases []string `json:"noun_phrases"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode parse response: %w", err)
	}

	return result.NounPhrases, nil
}
