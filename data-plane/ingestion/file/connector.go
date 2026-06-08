package file

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/chunker"
	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/embedding"
	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/ingestion"
	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/qdrant"
)

const (
	maxFileSize   = 50 << 20 // 50 MB
	maxEmbedBatch = 256
)

type DocTracker interface {
	GetDocHash(sourceID, docID string) (string, error)
	SetDocHash(sourceID, docID, hash string) error
	ListDocIDs(sourceID string) ([]string, error)
	DeleteDocTracking(sourceID, docID string) error
}

type Connector struct {
	mu         sync.RWMutex
	embedder   *embedding.Client
	vector     *qdrant.Client
	allowedDir string
	sources    map[string]ingestion.SourceConfig
	tracker    DocTracker
}

func New(embedder *embedding.Client, vector *qdrant.Client, allowedDir string) *Connector {
	return &Connector{
		embedder:   embedder,
		vector:     vector,
		allowedDir: allowedDir,
		sources:    make(map[string]ingestion.SourceConfig),
	}
}

func (c *Connector) SetTracker(t DocTracker) { c.tracker = t }

func (c *Connector) Connect(_ context.Context, cfg ingestion.SourceConfig) error {
	path := cfg.Config["path"]
	if path == "" {
		return fmt.Errorf("file connector: config.path is required")
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("file connector: resolve path %s: %w", path, err)
	}
	resolved, err = filepath.EvalSymlinks(resolved)
	if err != nil {
		return fmt.Errorf("file connector: eval symlinks %s: %w", path, err)
	}
	if c.allowedDir != "" {
		prefix := c.allowedDir + string(filepath.Separator)
		if resolved != c.allowedDir && !strings.HasPrefix(resolved, prefix) {
			return fmt.Errorf("file connector: path %s is outside allowed directory %s", resolved, c.allowedDir)
		}
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return fmt.Errorf("file connector: stat %s: %w", resolved, err)
	}
	if !info.IsDir() && info.Size() > maxFileSize {
		return fmt.Errorf("file connector: %s exceeds %d byte limit", resolved, maxFileSize)
	}
	cfg.Config["path"] = resolved
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

	seenDocs := make(map[string]bool, len(files))
	result := &ingestion.IngestResult{}

	type pendingChunk struct {
		path     string
		docID    string
		chunk    chunker.Chunk
		sourceID string
	}
	var pending []pendingChunk
	docHashes := make(map[string]string)

	for _, f := range files {
		docID := filepath.Base(f)
		seenDocs[docID] = true

		data, err := os.ReadFile(f)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: read: %v", docID, err))
			continue
		}
		if int64(len(data)) > maxFileSize {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: exceeds %d byte limit", docID, maxFileSize))
			continue
		}

		contentHash := fmt.Sprintf("%x", sha256.Sum256(data))
		if c.tracker != nil {
			prev, err := c.tracker.GetDocHash(sourceID, docID)
			if err == nil && prev == contentHash {
				result.DocumentsProcessed++
				continue
			}
		}

		chunks := chunker.Split(string(data), chunker.Options{})
		if len(chunks) == 0 {
			result.DocumentsProcessed++
			continue
		}

		docHashes[docID] = contentHash
		for _, ch := range chunks {
			pending = append(pending, pendingChunk{
				path:     f,
				docID:    docID,
				chunk:    ch,
				sourceID: sourceID,
			})
		}
		result.DocumentsProcessed++
	}

	for i := 0; i < len(pending); i += maxEmbedBatch {
		end := i + maxEmbedBatch
		if end > len(pending) {
			end = len(pending)
		}
		batch := pending[i:end]

		texts := make([]string, len(batch))
		for j, p := range batch {
			texts[j] = p.chunk.Text
		}

		vectors, err := c.embedder.Embed(ctx, texts)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("embed batch %d–%d: %v", i, end-1, err))
			continue
		}

		points := make([]qdrant.Point, len(batch))
		for j, p := range batch {
			hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", p.sourceID, p.path, p.chunk.Index)))
			points[j] = qdrant.Point{
				ID:     fmt.Sprintf("%x", hash[:16]),
				Vector: vectors[j],
				Payload: map[string]any{
					"text":      p.chunk.Text,
					"source_id": p.sourceID,
					"file":      p.docID,
					"chunk_idx": p.chunk.Index,
				},
			}
		}

		if err := c.vector.Upsert(ctx, cfg.Collection, points); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("upsert batch %d–%d: %v", i, end-1, err))
			continue
		}
		result.ChunksStored += len(points)
	}

	for docID, hash := range docHashes {
		if c.tracker != nil {
			_ = c.tracker.SetDocHash(sourceID, docID, hash)
		}
	}

	if c.tracker != nil {
		prevDocs, err := c.tracker.ListDocIDs(sourceID)
		if err == nil {
			for _, docID := range prevDocs {
				if !seenDocs[docID] {
					hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:delete", sourceID, docID)))
					pointID := fmt.Sprintf("%x", hash[:16])
					_ = c.vector.DeleteByFilter(ctx, cfg.Collection, qdrant.Filter{
						Must: []qdrant.Condition{{
							Key: "source_id", Match: &qdrant.Match{Value: sourceID},
						}, {
							Key: "file", Match: &qdrant.Match{Value: docID},
						}},
					})
					_ = pointID
					_ = c.tracker.DeleteDocTracking(sourceID, docID)
				}
			}
		}
	}

	result.Duration = time.Since(start)
	return result, nil
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
