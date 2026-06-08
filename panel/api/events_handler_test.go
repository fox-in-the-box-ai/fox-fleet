package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/events"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
)

func TestEvents_Empty(t *testing.T) {
	env := newTestEnv(t)
	w := env.doRequest("GET", "/api/events", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var evts []any
	if err := json.NewDecoder(w.Body).Decode(&evts); err != nil {
		t.Fatal(err)
	}
	if len(evts) != 0 {
		t.Fatalf("expected 0 events, got %d", len(evts))
	}
}

func TestEvents_WithLog(t *testing.T) {
	dir := t.TempDir()
	reg, err := registry.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Close() })

	elog := events.NewLog(100)
	elog.Emit("provision", "fox-1", "started")
	elog.Emit("destroy", "fox-2", "destroyed")

	srv := NewServer(Deps{
		Registry:     reg,
		Provisioner:  &fakeProvisioner{provisionCh: make(chan struct{}, 1)},
		Plugin:       &fakePlugin{},
		AdminSecret:  testSecret,
		InstancePwd:  "test-pwd",
		MaxInstances: 2,
		EventLog:     elog,
		SigningKey:   testSigningKey,
	})

	env := &testEnv{server: srv, registry: reg}
	w := env.doRequest("GET", "/api/events", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var evts []events.Event
	if err := json.NewDecoder(w.Body).Decode(&evts); err != nil {
		t.Fatal(err)
	}
	if len(evts) != 2 {
		t.Fatalf("expected 2 events, got %d", len(evts))
	}
	if evts[0].Type != "destroy" {
		t.Errorf("latest event type = %q, want destroy", evts[0].Type)
	}
	if evts[1].Type != "provision" {
		t.Errorf("oldest event type = %q, want provision", evts[1].Type)
	}
}

func TestEvents_AuthRequired(t *testing.T) {
	env := newTestEnv(t)
	w := env.doRequestNoAuth("GET", "/api/events")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
