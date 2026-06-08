package qdrant

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

type Timeouts struct {
	Health     time.Duration
	Search     time.Duration
	Upsert     time.Duration
	Collection time.Duration
}

func DefaultTimeouts() Timeouts {
	return Timeouts{
		Health:     5 * time.Second,
		Search:     30 * time.Second,
		Upsert:     60 * time.Second,
		Collection: 30 * time.Second,
	}
}

type Client struct {
	baseURL  string
	client   *http.Client
	timeouts Timeouts
}

func NewClient(baseURL string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:  baseURL,
		client:   &http.Client{},
		timeouts: DefaultTimeouts(),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

type ClientOption func(*Client)

func WithTimeouts(t Timeouts) ClientOption {
	return func(c *Client) { c.timeouts = t }
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

func (c *Client) withTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}

func (c *Client) EnsureCollection(ctx context.Context, name string, vectorSize int) error {
	ctx, cancel := c.withTimeout(ctx, c.timeouts.Collection)
	defer cancel()

	body, _ := json.Marshal(map[string]any{
		"vectors": map[string]any{
			"size":     vectorSize,
			"distance": "Cosine",
		},
	})

	url := fmt.Sprintf("%s/collections/%s", c.baseURL, url.PathEscape(name))
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
	ctx, cancel := c.withTimeout(ctx, c.timeouts.Upsert)
	defer cancel()

	const batchSize = 100
	if len(points) > batchSize {
		for i := 0; i < len(points); i += batchSize {
			end := i + batchSize
			if end > len(points) {
				end = len(points)
			}
			if err := c.upsertBatch(ctx, collection, points[i:end]); err != nil {
				return fmt.Errorf("batch %d–%d: %w", i, end-1, err)
			}
		}
		return nil
	}
	return c.upsertBatch(ctx, collection, points)
}

func (c *Client) upsertBatch(ctx context.Context, collection string, points []Point) error {
	body, err := json.Marshal(UpsertRequest{Points: points})
	if err != nil {
		return fmt.Errorf("qdrant: marshal upsert: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points", c.baseURL, url.PathEscape(collection))
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
	ctx, cancel := c.withTimeout(ctx, c.timeouts.Search)
	defer cancel()

	body, err := json.Marshal(sr)
	if err != nil {
		return nil, fmt.Errorf("qdrant: marshal search: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/search", c.baseURL, url.PathEscape(collection))
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

func (c *Client) DeleteByFilter(ctx context.Context, collection string, filter Filter) error {
	ctx, cancel := c.withTimeout(ctx, c.timeouts.Upsert)
	defer cancel()

	body, err := json.Marshal(map[string]any{"filter": filter})
	if err != nil {
		return fmt.Errorf("qdrant: marshal delete filter: %w", err)
	}
	u := fmt.Sprintf("%s/collections/%s/points/delete", c.baseURL, url.PathEscape(collection))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("qdrant: delete by filter request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant: delete by filter: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qdrant: delete by filter returned %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) DeleteCollection(ctx context.Context, name string) error {
	ctx, cancel := c.withTimeout(ctx, c.timeouts.Collection)
	defer cancel()

	url := fmt.Sprintf("%s/collections/%s", c.baseURL, url.PathEscape(name))
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
	ctx, cancel := c.withTimeout(ctx, c.timeouts.Health)
	defer cancel()

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
