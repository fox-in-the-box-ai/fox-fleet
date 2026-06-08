package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	client  *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

type Point struct {
	ID      string         `json:"id"`
	Vector  []float32      `json:"vector"`
	Payload map[string]any `json:"payload,omitempty"`
}

type UpsertRequest struct {
	Points []Point `json:"points"`
}

type SearchRequest struct {
	Vector      []float32 `json:"vector"`
	Limit       int       `json:"limit"`
	Filter      *Filter   `json:"filter,omitempty"`
	WithPayload bool      `json:"with_payload"`
}

type Filter struct {
	Must []Condition `json:"must,omitempty"`
}

type Condition struct {
	Key   string `json:"key"`
	Match *Match `json:"match,omitempty"`
}

type Match struct {
	Value string `json:"value,omitempty"`
}

type SearchResult struct {
	ID      string         `json:"id"`
	Score   float32        `json:"score"`
	Payload map[string]any `json:"payload,omitempty"`
}

type searchResponse struct {
	Result []SearchResult `json:"result"`
}

func (c *Client) EnsureCollection(ctx context.Context, name string, vectorSize int) error {
	body, _ := json.Marshal(map[string]any{
		"vectors": map[string]any{
			"size":     vectorSize,
			"distance": "Cosine",
		},
	})

	url := fmt.Sprintf("%s/collections/%s", c.baseURL, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("qdrant: create collection request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant: create collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qdrant: create collection returned %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) Upsert(ctx context.Context, collection string, points []Point) error {
	body, err := json.Marshal(UpsertRequest{Points: points})
	if err != nil {
		return fmt.Errorf("qdrant: marshal upsert: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points", c.baseURL, collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("qdrant: upsert request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant: upsert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qdrant: upsert returned %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) Search(ctx context.Context, collection string, sr SearchRequest) ([]SearchResult, error) {
	body, err := json.Marshal(sr)
	if err != nil {
		return nil, fmt.Errorf("qdrant: marshal search: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/search", c.baseURL, collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("qdrant: search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qdrant: search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qdrant: search returned %d", resp.StatusCode)
	}

	var result searchResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 50<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("qdrant: decode search: %w", err)
	}
	return result.Result, nil
}

func (c *Client) DeleteCollection(ctx context.Context, name string) error {
	url := fmt.Sprintf("%s/collections/%s", c.baseURL, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("qdrant: delete collection request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant: delete collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("qdrant: delete collection returned %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) Healthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return false
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
