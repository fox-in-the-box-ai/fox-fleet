package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/embedding"
	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/qdrant"
	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/source"

	_ "modernc.org/sqlite"
)

const testSecret = "test-admin-secret"

func setup(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	sources, err := source.OpenRegistry(db)
	if err != nil {
		t.Fatal(err)
	}

	embedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := struct {
			Data []struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			} `json:"data"`
		}{
			Data: []struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{{Embedding: []float32{0.1, 0.2, 0.3}, Index: 0}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(embedSrv.Close)

	qdrantSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(qdrantSrv.Close)

	ec := embedding.NewClient(embedding.Config{BaseURL: embedSrv.URL, Model: "test"})
	qc := qdrant.NewClient(qdrantSrv.URL)

	srv := New(Config{
		AdminSecret: testSecret,
		Collection:  "test-docs",
		VectorSize:  3,
	}, sources, ec, qc, nil)

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return srv, ts
}

func adminReq(method, url string, body any) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, url, &buf)
	req.Header.Set("Authorization", "Bearer "+testSecret)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestHealthEndpoint(t *testing.T) {
	_, ts := setup(t)

	resp, err := http.Get(ts.URL + "/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestReadyzEndpoint(t *testing.T) {
	_, ts := setup(t)

	resp, err := http.Get(ts.URL + "/v1/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestAdminRequiresAuth(t *testing.T) {
	_, ts := setup(t)

	resp, err := http.Get(ts.URL + "/v1/admin/sources")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestAdminBadToken(t *testing.T) {
	_, ts := setup(t)

	req, _ := http.NewRequest("GET", ts.URL+"/v1/admin/sources", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestSourceCRUD(t *testing.T) {
	_, ts := setup(t)

	// List — empty
	resp, err := http.DefaultClient.Do(adminReq("GET", ts.URL+"/v1/admin/sources", nil))
	if err != nil {
		t.Fatal(err)
	}
	var list []source.Source
	_ = json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if len(list) != 0 {
		t.Errorf("initial list = %d, want 0", len(list))
	}

	// Create
	body := map[string]any{
		"id":   "src-1",
		"type": "file",
		"name": "Test Source",
		"config": map[string]string{
			"path": "/tmp/test-docs",
		},
	}
	resp, err = http.DefaultClient.Do(adminReq("POST", ts.URL+"/v1/admin/sources", body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want 201", resp.StatusCode)
	}
	var created source.Source
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if created.ID != "src-1" {
		t.Errorf("created ID = %q", created.ID)
	}
	if created.Collection != "test-docs" {
		t.Errorf("collection = %q, want test-docs", created.Collection)
	}

	// Get
	resp, err = http.DefaultClient.Do(adminReq("GET", ts.URL+"/v1/admin/sources/src-1", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("get status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// List — 1 source
	resp, err = http.DefaultClient.Do(adminReq("GET", ts.URL+"/v1/admin/sources", nil))
	if err != nil {
		t.Fatal(err)
	}
	_ = json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if len(list) != 1 {
		t.Errorf("list = %d, want 1", len(list))
	}

	// Delete
	resp, err = http.DefaultClient.Do(adminReq("DELETE", ts.URL+"/v1/admin/sources/src-1", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("delete status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Get — not found
	resp, err = http.DefaultClient.Do(adminReq("GET", ts.URL+"/v1/admin/sources/src-1", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestCreateSourceValidation(t *testing.T) {
	_, ts := setup(t)

	tests := []struct {
		name string
		body map[string]any
		want int
	}{
		{"missing id", map[string]any{"type": "file", "name": "x"}, 400},
		{"missing type", map[string]any{"id": "x", "name": "x"}, 400},
		{"missing name", map[string]any{"id": "x", "type": "file"}, 400},
		{"bad type", map[string]any{"id": "x", "type": "grpc", "name": "x"}, 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.DefaultClient.Do(adminReq("POST", ts.URL+"/v1/admin/sources", tt.body))
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != tt.want {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.want)
			}
		})
	}
}

func TestPublicSourcesList(t *testing.T) {
	_, ts := setup(t)

	resp, err := http.Get(ts.URL + "/v1/sources")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	var list []source.Source
	_ = json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 0 {
		t.Errorf("initial list = %d, want 0", len(list))
	}
}

func TestDeleteNotFound(t *testing.T) {
	_, ts := setup(t)

	resp, err := http.DefaultClient.Do(adminReq("DELETE", ts.URL+"/v1/admin/sources/nonexistent", nil))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}
