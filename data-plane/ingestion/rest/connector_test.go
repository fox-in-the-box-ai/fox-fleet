package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

	qdrantSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(qdrantSrv.Close)

	ec := embedding.NewClient(embedding.Config{BaseURL: embedSrv.URL, Model: "test"})
	qc := qdrant.NewClient(qdrantSrv.URL)
	return ec, qc
}

func TestConnect_MissingURL(t *testing.T) {
	ec, qc := newTestServers(t)
	c := New(ec, qc)

	err := c.Connect(context.Background(), ingestion.SourceConfig{
		SourceID:   "s1",
		Collection: "docs",
		Config:     map[string]string{},
	})
	if err == nil {
		t.Fatal("expected error for missing url")
	}
}

func TestIngestSinglePage(t *testing.T) {
	ec, qc := newTestServers(t)

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		resp := pageResponse{
			Documents: []document{
				{ID: "d1", Text: "first document content"},
				{ID: "d2", Text: "second document content"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer apiSrv.Close()

	c := New(ec, qc)
	cfg := ingestion.SourceConfig{
		SourceID:    "s1",
		Collection:  "docs",
		Credentials: map[string]string{"bearer_token": "test-token"},
		Config:      map[string]string{"url": apiSrv.URL + "/docs"},
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
	if result.ChunksStored < 2 {
		t.Errorf("chunks = %d, want >= 2", result.ChunksStored)
	}
}

func TestIngestPaginated(t *testing.T) {
	ec, qc := newTestServers(t)

	page := 0
	var apiSrv *httptest.Server
	apiSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		resp := pageResponse{
			Documents: []document{
				{ID: "d" + r.URL.Path, Text: "document content"},
			},
		}
		if page == 1 {
			resp.NextURL = apiSrv.URL + "/page2"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer apiSrv.Close()

	c := New(ec, qc)
	cfg := ingestion.SourceConfig{
		SourceID:   "s1",
		Collection: "docs",
		Config:     map[string]string{"url": apiSrv.URL + "/page1"},
	}
	_ = c.Connect(context.Background(), cfg)

	result, err := c.Ingest(context.Background(), "s1")
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if result.DocumentsProcessed != 2 {
		t.Errorf("docs = %d, want 2", result.DocumentsProcessed)
	}
}

func TestIngestAPIError(t *testing.T) {
	ec, qc := newTestServers(t)

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer apiSrv.Close()

	c := New(ec, qc)
	cfg := ingestion.SourceConfig{
		SourceID:   "s1",
		Collection: "docs",
		Config:     map[string]string{"url": apiSrv.URL + "/docs"},
	}
	_ = c.Connect(context.Background(), cfg)

	result, err := c.Ingest(context.Background(), "s1")
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if len(result.Errors) == 0 {
		t.Error("expected errors for 403 response")
	}
}

func TestIngestNotConnected(t *testing.T) {
	ec, qc := newTestServers(t)
	c := New(ec, qc)

	_, err := c.Ingest(context.Background(), "unknown")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestStatusAndDisconnect(t *testing.T) {
	ec, qc := newTestServers(t)
	c := New(ec, qc)

	cfg := ingestion.SourceConfig{
		SourceID:   "s1",
		Collection: "docs",
		Config:     map[string]string{"url": "http://example.com/docs"},
	}
	_ = c.Connect(context.Background(), cfg)

	st, _ := c.Status(context.Background(), "s1")
	if !st.Connected {
		t.Error("expected connected")
	}

	_ = c.Disconnect(context.Background(), "s1")
	st, _ = c.Status(context.Background(), "s1")
	if st.Connected {
		t.Error("expected disconnected")
	}
}
