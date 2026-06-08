package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestSmokeLifecycle(t *testing.T) {
	env, _ := newSkillsetTestEnv(t)

	w := env.doRequest("GET", "/api/instances", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list instances: expected 200, got %d", w.Code)
	}
	var empty []instanceItem
	if err := json.NewDecoder(w.Body).Decode(&empty); err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected 0 instances at start, got %d", len(empty))
	}

	w = env.doRequest("POST", "/api/instances", `{"id":"smoke-1"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create instance: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created createResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.ID != "smoke-1" {
		t.Fatalf("created ID = %q, want smoke-1", created.ID)
	}

	select {
	case <-env.prov.provisionCh:
	case <-time.After(2 * time.Second):
		t.Fatal("provisioner not called within timeout")
	}

	env.seedInstance(t, "smoke-1", 9200)

	w = env.doRequest("GET", "/api/instances", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list after create: expected 200, got %d", w.Code)
	}
	var list []instanceItem
	if err := json.NewDecoder(w.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range list {
		if item.ID == "smoke-1" {
			found = true
		}
	}
	if !found {
		t.Fatal("smoke-1 not found in instance list after seeding")
	}

	w = env.doRequest("GET", "/api/instances/smoke-1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("detail: expected 200, got %d", w.Code)
	}

	w = uploadMultipart(t, env, validSkillsetYAML)
	if w.Code != http.StatusCreated {
		t.Fatalf("upload skillset: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/skillsets", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list skillsets: expected 200, got %d", w.Code)
	}
	var skillsets []skillsetSummary
	if err := json.NewDecoder(w.Body).Decode(&skillsets); err != nil {
		t.Fatal(err)
	}
	if len(skillsets) != 1 || skillsets[0].Name != "test-skillset" {
		t.Fatalf("expected [test-skillset], got %v", skillsets)
	}

	w = env.doRequest("GET", "/api/skillsets/test-skillset", "")
	if w.Code != http.StatusOK {
		t.Fatalf("get skillset: expected 200, got %d", w.Code)
	}

	w = env.doRequest("GET", "/api/skillsets/test-skillset/download", "")
	if w.Code != http.StatusOK {
		t.Fatalf("download skillset: expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/x-yaml" {
		t.Errorf("download Content-Type = %q, want application/x-yaml", ct)
	}

	w = env.doRequest("DELETE", "/api/skillsets/test-skillset", "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete skillset: expected 204, got %d", w.Code)
	}

	w = env.doRequest("GET", "/api/skillsets", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list after skillset delete: expected 200, got %d", w.Code)
	}
	var emptySkillsets []skillsetSummary
	if err := json.NewDecoder(w.Body).Decode(&emptySkillsets); err != nil {
		t.Fatal(err)
	}
	if len(emptySkillsets) != 0 {
		t.Fatalf("expected 0 skillsets after delete, got %d", len(emptySkillsets))
	}

	w = env.doRequest("GET", "/api/events", "")
	if w.Code != http.StatusOK {
		t.Fatalf("events: expected 200, got %d", w.Code)
	}

	w = env.doRequest("DELETE", "/api/instances/smoke-1", "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("destroy: expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSmokeAuthGate(t *testing.T) {
	env, _ := newSkillsetTestEnv(t)

	paths := []struct {
		method string
		path   string
	}{
		{"GET", "/api/instances"},
		{"POST", "/api/instances"},
		{"GET", "/api/instances/x"},
		{"DELETE", "/api/instances/x"},
		{"GET", "/api/sources"},
		{"GET", "/api/skillsets"},
		{"GET", "/api/skillsets/x"},
		{"GET", "/api/skillsets/x/download"},
		{"DELETE", "/api/skillsets/x"},
		{"POST", "/api/query"},
		{"GET", "/api/events"},
	}

	for _, p := range paths {
		w := env.doRequestNoAuth(p.method, p.path)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without auth: expected 401, got %d", p.method, p.path, w.Code)
		}
	}

	w := env.doRequestNoAuth("GET", "/healthz")
	if w.Code != http.StatusOK {
		t.Errorf("GET /healthz should be unauthenticated, got %d", w.Code)
	}
}
