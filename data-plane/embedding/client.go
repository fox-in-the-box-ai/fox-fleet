package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"time"
)

type Config struct {
	BaseURL        string
	APIKey         string
	Model          string
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

type Client struct {
	cfg    Config
	client *http.Client
}

func NewClient(cfg Config) *Client {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = 1 * time.Second
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 10 * time.Second
	}
	return &Client{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

type embeddingRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	body, err := json.Marshal(embeddingRequest{
		Input: texts,
		Model: c.cfg.Model,
	})
	if err != nil {
		return nil, fmt.Errorf("embedding: marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := c.backoff(attempt)
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("embedding: %w (after %d attempts, last error: %v)", ctx.Err(), attempt, lastErr)
			case <-time.After(backoff):
			}
		}

		result, err := c.doEmbed(ctx, body)
		if err == nil {
			vectors := make([][]float32, len(texts))
			for _, d := range result.Data {
				if d.Index < len(vectors) {
					vectors[d.Index] = d.Embedding
				}
			}
			return vectors, nil
		}

		lastErr = err
		if !isRetryable(err) {
			return nil, err
		}
	}

	return nil, lastErr
}

func (c *Client) doEmbed(ctx context.Context, body []byte) (*embeddingResponse, error) {
	url := c.cfg.BaseURL + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embedding: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &statusError{Code: resp.StatusCode}
	}

	var result embeddingResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 50<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("embedding: decode response: %w", err)
	}
	return &result, nil
}

type statusError struct {
	Code int
}

func (e *statusError) Error() string {
	return fmt.Sprintf("embedding: API returned %d", e.Code)
}

func isRetryable(err error) bool {
	var se *statusError
	if errors.As(err, &se) {
		switch se.Code {
		case 429, 500, 502, 503, 504:
			return true
		}
		return false
	}
	return true
}

func (c *Client) backoff(attempt int) time.Duration {
	b := float64(c.cfg.InitialBackoff) * math.Pow(2, float64(attempt-1))
	if b > float64(c.cfg.MaxBackoff) {
		b = float64(c.cfg.MaxBackoff)
	}
	jitter := b * 0.5 * rand.Float64()
	return time.Duration(b + jitter)
}
