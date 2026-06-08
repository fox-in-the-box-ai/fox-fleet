package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEmbed_Empty(t *testing.T) {
	c := NewClient(Config{})
	vecs, err := c.Embed(context.Background(), nil)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if vecs != nil {
		t.Errorf("vecs = %v, want nil", vecs)
	}
}

func TestEmbed_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Errorf("path = %q, want /embeddings", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}

		var req embeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req.Model != "text-embedding-3-small" {
			t.Errorf("model = %q", req.Model)
		}

		resp := embeddingResponse{
			Data: []struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{Embedding: []float32{0.1, 0.2, 0.3}, Index: 0},
				{Embedding: []float32{0.4, 0.5, 0.6}, Index: 1},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(Config{
		BaseURL: srv.URL,
		APIKey:  "test-key",
		Model:   "text-embedding-3-small",
	})

	vecs, err := c.Embed(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("len(vecs) = %d, want 2", len(vecs))
	}
	if len(vecs[0]) != 3 {
		t.Errorf("len(vecs[0]) = %d, want 3", len(vecs[0]))
	}
}

func TestEmbed_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, Model: "test", MaxRetries: 1})
	_, err := c.Embed(context.Background(), []string{"hello"})
	if err == nil {
		t.Fatal("expected error for 429 response")
	}
}

func TestEmbed_RetryThenSuccess(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		resp := embeddingResponse{
			Data: []struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{Embedding: []float32{0.1}, Index: 0},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(Config{
		BaseURL:        srv.URL,
		Model:          "test",
		MaxRetries:     3,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     5 * time.Millisecond,
	})
	vecs, err := c.Embed(context.Background(), []string{"hello"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 1 {
		t.Errorf("len(vecs) = %d, want 1", len(vecs))
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestEmbed_NoRetryOn400(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	c := NewClient(Config{
		BaseURL:        srv.URL,
		Model:          "test",
		MaxRetries:     3,
		InitialBackoff: time.Millisecond,
	})
	_, err := c.Embed(context.Background(), []string{"hello"})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (should not retry on 400)", calls)
	}
}

func TestEmbed_NoAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("Authorization header should be absent without API key")
		}
		resp := embeddingResponse{
			Data: []struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{Embedding: []float32{0.1}, Index: 0},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, Model: "local"})
	vecs, err := c.Embed(context.Background(), []string{"hello"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 1 {
		t.Errorf("len(vecs) = %d, want 1", len(vecs))
	}
}
