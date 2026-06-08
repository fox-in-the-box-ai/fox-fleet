package qdrant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnsureCollection_Created(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/collections/docs" {
			t.Errorf("path = %q, want /collections/docs", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		vectors, ok := body["vectors"].(map[string]any)
		if !ok {
			t.Fatal("missing vectors in body")
		}
		if vectors["distance"] != "Cosine" {
			t.Errorf("distance = %v", vectors["distance"])
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.EnsureCollection(context.Background(), "docs", 384); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
}

func TestEnsureCollection_AlreadyExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.EnsureCollection(context.Background(), "docs", 384); err != nil {
		t.Fatalf("EnsureCollection should succeed on 409: %v", err)
	}
}

func TestEnsureCollection_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.EnsureCollection(context.Background(), "docs", 384); err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestUpsert_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/collections/docs/points" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var req UpsertRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(req.Points) != 1 {
			t.Errorf("len(points) = %d, want 1", len(req.Points))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	err := c.Upsert(context.Background(), "docs", []Point{
		{ID: "p1", Vector: []float32{0.1, 0.2}, Payload: map[string]any{"src": "test"}},
	})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
}

func TestUpsert_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	err := c.Upsert(context.Background(), "docs", []Point{
		{ID: "p1", Vector: []float32{0.1}},
	})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
}

func TestSearch_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/collections/docs/points/search" {
			t.Errorf("path = %q", r.URL.Path)
		}
		resp := searchResponse{
			Result: []SearchResult{
				{ID: "p1", Score: 0.95, Payload: map[string]any{"text": "hello"}},
				{ID: "p2", Score: 0.80},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	results, err := c.Search(context.Background(), "docs", SearchRequest{
		Vector:      []float32{0.1, 0.2},
		Limit:       5,
		WithPayload: true,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].ID != "p1" {
		t.Errorf("results[0].ID = %q, want p1", results[0].ID)
	}
	if results[0].Score != 0.95 {
		t.Errorf("results[0].Score = %f, want 0.95", results[0].Score)
	}
}

func TestSearch_WithFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var sr SearchRequest
		if err := json.NewDecoder(r.Body).Decode(&sr); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if sr.Filter == nil || len(sr.Filter.Must) == 0 {
			t.Fatal("expected filter")
		}
		if sr.Filter.Must[0].Key != "source_id" {
			t.Errorf("filter key = %q", sr.Filter.Must[0].Key)
		}
		resp := searchResponse{Result: []SearchResult{{ID: "p1", Score: 0.9}}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	results, err := c.Search(context.Background(), "docs", SearchRequest{
		Vector: []float32{0.1},
		Limit:  3,
		Filter: &Filter{Must: []Condition{
			{Key: "source_id", Match: &Match{Value: "src-1"}},
		}},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
}

func TestSearch_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.Search(context.Background(), "docs", SearchRequest{
		Vector: []float32{0.1},
		Limit:  5,
	})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestDeleteCollection_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/collections/docs" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.DeleteCollection(context.Background(), "docs"); err != nil {
		t.Fatalf("DeleteCollection: %v", err)
	}
}

func TestDeleteCollection_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.DeleteCollection(context.Background(), "docs"); err != nil {
		t.Fatalf("DeleteCollection should succeed on 404: %v", err)
	}
}

func TestHealthy_Up(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Errorf("path = %q, want /healthz", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if !c.Healthy(context.Background()) {
		t.Error("expected healthy")
	}
}

func TestHealthy_Down(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if c.Healthy(context.Background()) {
		t.Error("expected unhealthy for 503")
	}
}

func TestHealthy_Unreachable(t *testing.T) {
	c := NewClient("http://127.0.0.1:1")
	if c.Healthy(context.Background()) {
		t.Error("expected unhealthy for unreachable server")
	}
}
