package file

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/embedding"
	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/ingestion"
	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/qdrant"
)

func newTestServers(t *testing.T) (*embedding.Client, *qdrant.Client) {
	t.Helper()

	embedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input []string `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		type embData struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		}
		resp := struct {
			Data []embData `json:"data"`
		}{}
		for i := range req.Input {
			resp.Data = append(resp.Data, embData{
				Embedding: []float32{0.1, 0.2, 0.3},
				Index:     i,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(embedSrv.Close)

	qdrantSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(qdrantSrv.Close)

	ec := embedding.NewClient(embedding.Config{BaseURL: embedSrv.URL, Model: "test"})
	qc := qdrant.NewClient(qdrantSrv.URL)
	return ec, qc
}

func TestConnect_MissingPath(t *testing.T) {
	ec, qc := newTestServers(t)
	c := New(ec, qc)

	err := c.Connect(context.Background(), ingestion.SourceConfig{
		SourceID:   "s1",
		Collection: "docs",
		Config:     map[string]string{},
	})
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestConnect_NonexistentPath(t *testing.T) {
	ec, qc := newTestServers(t)
	c := New(ec, qc)

	err := c.Connect(context.Background(), ingestion.SourceConfig{
		SourceID:   "s1",
		Collection: "docs",
		Config:     map[string]string{"path": "/nonexistent/path/xyz"},
	})
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestIngestSingleFile(t *testing.T) {
	ec, qc := newTestServers(t)
	c := New(ec, qc)

	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	_ = os.WriteFile(f, []byte("hello world"), 0644)

	cfg := ingestion.SourceConfig{
		SourceID:   "s1",
		Collection: "docs",
		Config:     map[string]string{"path": f},
	}
	if err := c.Connect(context.Background(), cfg); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	result, err := c.Ingest(context.Background(), "s1")
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if result.DocumentsProcessed != 1 {
		t.Errorf("docs = %d, want 1", result.DocumentsProcessed)
	}
	if result.ChunksStored < 1 {
		t.Errorf("chunks = %d, want >= 1", result.ChunksStored)
	}
	if len(result.Errors) != 0 {
		t.Errorf("errors = %v", result.Errors)
	}
}

func TestIngestDirectory(t *testing.T) {
	ec, qc := newTestServers(t)
	c := New(ec, qc)

	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("document one"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "b.md"), []byte("document two"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "c.jpg"), []byte("not a text file"), 0644) // skipped

	cfg := ingestion.SourceConfig{
		SourceID:   "s1",
		Collection: "docs",
		Config:     map[string]string{"path": dir},
	}
	if err := c.Connect(context.Background(), cfg); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	result, err := c.Ingest(context.Background(), "s1")
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if result.DocumentsProcessed != 2 {
		t.Errorf("docs = %d, want 2", result.DocumentsProcessed)
	}
}

func TestIngestNotConnected(t *testing.T) {
	ec, qc := newTestServers(t)
	c := New(ec, qc)

	_, err := c.Ingest(context.Background(), "unknown")
	if err == nil {
		t.Fatal("expected error for unconnected source")
	}
}

func TestStatusAndDisconnect(t *testing.T) {
	ec, qc := newTestServers(t)
	c := New(ec, qc)

	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644)

	cfg := ingestion.SourceConfig{
		SourceID:   "s1",
		Collection: "docs",
		Config:     map[string]string{"path": filepath.Join(dir, "test.txt")},
	}
	_ = c.Connect(context.Background(), cfg)

	st, err := c.Status(context.Background(), "s1")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Connected {
		t.Error("expected connected")
	}

	_ = c.Disconnect(context.Background(), "s1")
	st, _ = c.Status(context.Background(), "s1")
	if st.Connected {
		t.Error("expected disconnected")
	}
}

func TestIngestEmptyFile(t *testing.T) {
	ec, qc := newTestServers(t)
	c := New(ec, qc)

	dir := t.TempDir()
	f := filepath.Join(dir, "empty.txt")
	_ = os.WriteFile(f, []byte(""), 0644)

	cfg := ingestion.SourceConfig{
		SourceID:   "s1",
		Collection: "docs",
		Config:     map[string]string{"path": f},
	}
	_ = c.Connect(context.Background(), cfg)

	result, err := c.Ingest(context.Background(), "s1")
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if result.DocumentsProcessed != 1 {
		t.Errorf("docs = %d, want 1", result.DocumentsProcessed)
	}
	if result.ChunksStored != 0 {
		t.Errorf("chunks = %d, want 0", result.ChunksStored)
	}
}
