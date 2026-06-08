package file

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/chunker"
	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/embedding"
	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/ingestion"
	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/qdrant"
)

const maxFileSize = 50 << 20 // 50 MB

type Connector struct {
	mu       sync.RWMutex
	embedder *embedding.Client
	vector   *qdrant.Client
	sources  map[string]ingestion.SourceConfig
}

func New(embedder *embedding.Client, vector *qdrant.Client) *Connector {
	return &Connector{
		embedder: embedder,
		vector:   vector,
		sources:  make(map[string]ingestion.SourceConfig),
	}
}

func (c *Connector) Connect(_ context.Context, cfg ingestion.SourceConfig) error {
	path := cfg.Config["path"]
	if path == "" {
		return fmt.Errorf("file connector: config.path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("file connector: stat %s: %w", path, err)
	}
	if !info.IsDir() && info.Size() > maxFileSize {
		return fmt.Errorf("file connector: %s exceeds %d byte limit", path, maxFileSize)
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
		return nil, fmt.Errorf("file connector: source %s not connected", sourceID)
	}

	start := time.Now()
	path := cfg.Config["path"]

	var files []string
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("file connector: stat %s: %w", path, err)
	}
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("file connector: read dir %s: %w", path, err)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := filepath.Ext(e.Name())
			if ext == ".txt" || ext == ".csv" || ext == ".md" {
				files = append(files, filepath.Join(path, e.Name()))
			}
		}
	} else {
		files = []string{path}
	}

	result := &ingestion.IngestResult{}
	for _, f := range files {
		n, err := c.ingestFile(ctx, f, cfg.Collection, sourceID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", filepath.Base(f), err))
			continue
		}
		result.DocumentsProcessed++
		result.ChunksStored += n
	}
	result.Duration = time.Since(start)
	return result, nil
}

func (c *Connector) ingestFile(ctx context.Context, path, collection, sourceID string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read: %w", err)
	}
	if int64(len(data)) > maxFileSize {
		return 0, fmt.Errorf("exceeds %d byte limit", maxFileSize)
	}

	text := string(data)
	chunks := chunker.Split(text, chunker.Options{})
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
		hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", sourceID, path, ch.Index)))
		points[i] = qdrant.Point{
			ID:     fmt.Sprintf("%x", hash[:16]),
			Vector: vectors[i],
			Payload: map[string]any{
				"text":      ch.Text,
				"source_id": sourceID,
				"file":      filepath.Base(path),
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
