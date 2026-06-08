package api

import (
	"encoding/json"
	"net/http"
	"testing"

	_ "modernc.org/sqlite"
)

func TestHealthz_NoAuth(t *testing.T) {
	env := newTestEnv(t)
	w := env.doRequestNoAuth("GET", "/healthz")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("expected status=ok, got %q", resp["status"])
	}
}
