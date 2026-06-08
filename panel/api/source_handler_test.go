package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/source"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
)

func newSourceTestEnv(t *testing.T) (*testEnv, *source.Registry) {
	t.Helper()

	dir := t.TempDir()
	reg, err := registry.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Close() })

	srcDB, err := sql.Open("sqlite", filepath.Join(dir, "sources.db"))
	if err != nil {
		t.Fatal(err)
	}
	srcDB.SetMaxOpenConns(1)
	t.Cleanup(func() { srcDB.Close() })

	srcReg, err := source.OpenRegistry(srcDB)
	if err != nil {
		t.Fatal(err)
	}

	srv := NewServer(Deps{
		Registry:       reg,
		Provisioner:    &fakeProvisioner{provisionCh: make(chan struct{}, 1)},
		Plugin:         &fakePlugin{},
		AdminSecret:    testSecret,
		InstancePwd:    "test-instance-pwd",
		MaxInstances:   2,
		SourceRegistry: srcReg,
	})

	return &testEnv{
		server:   srv,
		registry: reg,
	}, srcReg
}

func TestGetSource_Found(t *testing.T) {
	env, srcReg := newSourceTestEnv(t)

	err := srcReg.Create(source.Source{
		ID:         "src-1",
		Type:       "file",
		Name:       "Test Docs",
		Collection: "knowledge",
		Config:     map[string]string{"path": "/data/docs"},
	})
	if err != nil {
		t.Fatal(err)
	}

	w := env.doRequest("GET", "/api/sources/src-1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var src source.Source
	if err := json.NewDecoder(w.Body).Decode(&src); err != nil {
		t.Fatal(err)
	}
	if src.ID != "src-1" {
		t.Errorf("id = %q, want src-1", src.ID)
	}
	if src.Name != "Test Docs" {
		t.Errorf("name = %q, want Test Docs", src.Name)
	}
	if src.Collection != "knowledge" {
		t.Errorf("collection = %q, want knowledge", src.Collection)
	}
	if src.Config["path"] != "/data/docs" {
		t.Errorf("config.path = %q, want /data/docs", src.Config["path"])
	}
}

func TestGetSource_NotFound(t *testing.T) {
	env, _ := newSourceTestEnv(t)

	w := env.doRequest("GET", "/api/sources/nonexistent", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}

	var resp apiError
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != "not_found" {
		t.Errorf("error = %q, want not_found", resp.Error)
	}
}

func TestGetSource_NoRegistry(t *testing.T) {
	env := newTestEnv(t)

	w := env.doRequest("GET", "/api/sources/any-id", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}
