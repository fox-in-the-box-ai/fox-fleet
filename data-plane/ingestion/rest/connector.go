package rest

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/chunker"
	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/embedding"
	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/ingestion"
	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/qdrant"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/safedialer"
)

const (
	maxResponseBody = 50 << 20 // 50 MB
	maxPages        = 1000
)

type Connector struct {
	mu       sync.RWMutex
	embedder *embedding.Client
	vector   *qdrant.Client
	http     *http.Client
	sources  map[string]ingestion.SourceConfig
}

func New(embedder *embedding.Client, vector *qdrant.Client) *Connector {
	return &Connector{
		embedder: embedder,
		vector:   vector,
		http: &http.Client{
			Timeout:   60 * time.Second,
			Transport: &http.Transport{DialContext: safedialer.New().DialContext},
		},
		sources: make(map[string]ingestion.SourceConfig),
	}
}

func NewWithClient(embedder *embedding.Client, vector *qdrant.Client, httpClient *http.Client) *Connector {
	return &Connector{
		embedder: embedder,
		vector:   vector,
		http:     httpClient,
		sources:  make(map[string]ingestion.SourceConfig),
	}
}

func (c *Connector) Connect(_ context.Context, cfg ingestion.SourceConfig) error {
	if cfg.Config["url"] == "" {
		return fmt.Errorf("rest connector: config.url is required")
	}
	c.mu.Lock()
	c.sources[cfg.SourceID] = cfg
	c.mu.Unlock()
	return nil
}

func (c *Connector) Ingest(ctx context.Context, sourceID string) (*ingestion.IngestResult, error) {
	c.mu.RLock()
	cfg, ok := c.sources[sourceID]
	c.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("rest connector: source %s not connected", sourceID)
	}

	start := time.Now()
	result := &ingestion.IngestResult{}

	originURL := cfg.Config["url"]
	origin, err := url.Parse(originURL)
	if err != nil {
		return nil, fmt.Errorf("rest connector: invalid origin url: %w", err)
	}

	cur := originURL
	for page := 0; cur != "" && page < maxPages; page++ {
		docs, nextURL, err := c.fetchPage(ctx, cur, cfg)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			break
		}

		for _, doc := range docs {
			n, err := c.ingestDocument(ctx, doc, cfg.Collection, sourceID)
			if err != nil {
				result.Errors = append(result.Errors, err.Error())
				continue
			}
			result.DocumentsProcessed++
			result.ChunksStored += n
		}

		if nextURL != "" {
			if err := validateNextURL(nextURL, origin); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("next_url rejected: %v", err))
				break
			}
		}
		cur = nextURL
		if page+1 >= maxPages && cur != "" {
			result.Errors = append(result.Errors, fmt.Sprintf("pagination limit reached (%d pages)", maxPages))
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

type document struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type pageResponse struct {
	Documents []document `json:"documents"`
	NextURL   string     `json:"next_url,omitempty"`
}

func (c *Connector) fetchPage(ctx context.Context, url string, cfg ingestion.SourceConfig) ([]document, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("rest: create request: %w", err)
	}
	if token := cfg.Credentials["bearer_token"]; token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("rest: fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("rest: %s returned %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, "", fmt.Errorf("rest: read body: %w", err)
	}

	var page pageResponse
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, "", fmt.Errorf("rest: decode page: %w", err)
	}
	return page.Documents, page.NextURL, nil
}

func (c *Connector) ingestDocument(ctx context.Context, doc document, collection, sourceID string) (int, error) {
	chunks := chunker.Split(doc.Text, chunker.Options{})
	if len(chunks) == 0 {
		return 0, nil
	}

	texts := make([]string, len(chunks))
	for i, ch := range chunks {
		texts[i] = ch.Text
	}

	vectors, err := c.embedder.Embed(ctx, texts)
	if err != nil {
		return 0, fmt.Errorf("embed: %w", err)
	}

	points := make([]qdrant.Point, len(chunks))
	for i, ch := range chunks {
		hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", sourceID, doc.ID, ch.Index)))
		points[i] = qdrant.Point{
			ID:     fmt.Sprintf("%x", hash[:16]),
			Vector: vectors[i],
			Payload: map[string]any{
				"text":      ch.Text,
				"source_id": sourceID,
				"doc_id":    doc.ID,
				"chunk_idx": ch.Index,
			},
		}
	}

	if err := c.vector.Upsert(ctx, collection, points); err != nil {
		return 0, fmt.Errorf("upsert: %w", err)
	}
	return len(points), nil
}

func (c *Connector) Status(_ context.Context, sourceID string) (*ingestion.SourceStatus, error) {
	c.mu.RLock()
	_, ok := c.sources[sourceID]
	c.mu.RUnlock()
	return &ingestion.SourceStatus{Connected: ok}, nil
}

func (c *Connector) Disconnect(_ context.Context, sourceID string) error {
	c.mu.Lock()
	delete(c.sources, sourceID)
	c.mu.Unlock()
	return nil
}

func validateNextURL(nextURL string, origin *url.URL) error {
	parsed, err := url.Parse(nextURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != origin.Scheme {
		return fmt.Errorf("scheme mismatch: %s vs %s", parsed.Scheme, origin.Scheme)
	}
	if parsed.Host != origin.Host {
		return fmt.Errorf("host mismatch: %s vs %s", parsed.Host, origin.Host)
	}
	return nil
}
