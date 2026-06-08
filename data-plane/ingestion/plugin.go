package ingestion

import (
	"context"
	"time"
)

type Plugin interface {
	Connect(ctx context.Context, cfg SourceConfig) error
	Ingest(ctx context.Context, sourceID string) (*IngestResult, error)
	Status(ctx context.Context, sourceID string) (*SourceStatus, error)
	Disconnect(ctx context.Context, sourceID string) error
}

type SourceConfig struct {
	SourceID    string            `json:"source_id"`
	Type        string            `json:"type"`
	Credentials map[string]string `json:"credentials,omitempty"`
	Schedule    string            `json:"schedule,omitempty"`
	Collection  string            `json:"collection"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Config      map[string]string `json:"config,omitempty"`
}

type IngestResult struct {
	DocumentsProcessed int           `json:"documents_processed"`
	ChunksStored       int           `json:"chunks_stored"`
	Errors             []string      `json:"errors,omitempty"`
	Duration           time.Duration `json:"duration_ns"`
}

type SourceStatus struct {
	Connected     bool      `json:"connected"`
	LastRefresh   time.Time `json:"last_refresh,omitempty"`
	NextRefresh   time.Time `json:"next_refresh,omitempty"`
	DocumentCount int       `json:"document_count"`
	ChunkCount    int       `json:"chunk_count"`
	ErrorDetail   string    `json:"error_detail,omitempty"`
}
