package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
)

const validSkillsetYAML = `name: test-skillset
version: "1.0.0"
contract_version: "2.0.0"
persona:
  system_prompt_file: prompt.md
tools:
  - name: web_search
    type: builtin
data_sources:
  - binding: knowledge
    query_mode: rag
memory:
  provider: mem0
capabilities:
  chat: true
  search: true
ui:
  branding:
    bot_name: TestBot
`

func newSkillsetTestEnv(t *testing.T) (*testEnv, string) {
	t.Helper()

	dir := t.TempDir()
	reg, err := registry.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Close() })

	skillsetsDir := filepath.Join(dir, "skillsets")

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	prov := &fakeProvisioner{provisionCh: make(chan struct{}, 1)}

	srv := NewServer(Deps{
		Registry:     reg,
		Provisioner:  prov,
		Plugin:       &fakePlugin{},
		AdminSecret:  testSecret,
		InstancePwd:  "test-instance-pwd",
		MaxInstances: 2,
		PollInterval: time.Hour,
		Logger:       logger,
		SkillsetsDir: skillsetsDir,
	})

	return &testEnv{
		server:   srv,
		registry: reg,
		prov:     prov,
		logBuf:   &logBuf,
	}, skillsetsDir
}

func uploadMultipart(t *testing.T, env *testEnv, content string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", "skillset.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(part, content); err != nil {
		t.Fatal(err)
	}
	w.Close()

	req := httptest.NewRequest("POST", "/api/skillsets", &buf)
	req.Header.Set("Authorization", "Bearer "+testSecret)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	env.server.Handler().ServeHTTP(rec, req)
	return rec
}

func TestListSkillsets_Empty(t *testing.T) {
	env, _ := newSkillsetTestEnv(t)
	w := env.doRequest("GET", "/api/skillsets", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var items []skillsetSummary
	if err := json.NewDecoder(w.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 skillsets, got %d", len(items))
	}
}

func TestUploadSkillset(t *testing.T) {
	env, _ := newSkillsetTestEnv(t)

	w := uploadMultipart(t, env, validSkillsetYAML)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var sk skillsetSummary
	if err := json.NewDecoder(w.Body).Decode(&sk); err != nil {
		t.Fatal(err)
	}
	if sk.Name != "test-skillset" {
		t.Errorf("name = %q, want test-skillset", sk.Name)
	}
	if sk.Version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", sk.Version)
	}
	if sk.ToolCount != 1 {
		t.Errorf("tool_count = %d, want 1", sk.ToolCount)
	}
	if sk.SourceCount != 1 {
		t.Errorf("source_count = %d, want 1", sk.SourceCount)
	}
}

func TestUploadSkillset_InvalidYAML(t *testing.T) {
	env, _ := newSkillsetTestEnv(t)
	w := uploadMultipart(t, env, "name: bad\nversion: not-semver\n")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListSkillsets_AfterUpload(t *testing.T) {
	env, _ := newSkillsetTestEnv(t)

	w := uploadMultipart(t, env, validSkillsetYAML)
	if w.Code != http.StatusCreated {
		t.Fatalf("upload: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/skillsets", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}
	var items []skillsetSummary
	if err := json.NewDecoder(w.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 skillset, got %d", len(items))
	}
	if items[0].Name != "test-skillset" {
		t.Errorf("name = %q, want test-skillset", items[0].Name)
	}
}

func TestGetSkillset(t *testing.T) {
	env, _ := newSkillsetTestEnv(t)

	w := uploadMultipart(t, env, validSkillsetYAML)
	if w.Code != http.StatusCreated {
		t.Fatalf("upload: expected 201, got %d", w.Code)
	}

	w = env.doRequest("GET", "/api/skillsets/test-skillset", "")
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var m map[string]any
	if err := json.NewDecoder(w.Body).Decode(&m); err != nil {
		t.Fatal(err)
	}
	if m["name"] != "test-skillset" {
		t.Errorf("name = %v", m["name"])
	}
	if m["version"] != "1.0.0" {
		t.Errorf("version = %v", m["version"])
	}
}

func TestGetSkillset_NotFound(t *testing.T) {
	env, _ := newSkillsetTestEnv(t)
	w := env.doRequest("GET", "/api/skillsets/nonexistent", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDeleteSkillset(t *testing.T) {
	env, skDir := newSkillsetTestEnv(t)

	w := uploadMultipart(t, env, validSkillsetYAML)
	if w.Code != http.StatusCreated {
		t.Fatalf("upload: expected 201, got %d", w.Code)
	}

	if _, err := os.Stat(filepath.Join(skDir, "test-skillset.yaml")); err != nil {
		t.Fatalf("skillset file should exist: %v", err)
	}

	w = env.doRequest("DELETE", "/api/skillsets/test-skillset", "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	if _, err := os.Stat(filepath.Join(skDir, "test-skillset.yaml")); !os.IsNotExist(err) {
		t.Fatal("skillset file should be deleted")
	}
}

func TestDeleteSkillset_NotFound(t *testing.T) {
	env, _ := newSkillsetTestEnv(t)
	w := env.doRequest("DELETE", "/api/skillsets/nonexistent", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadSkillset_MissingFile(t *testing.T) {
	env, _ := newSkillsetTestEnv(t)
	w := env.doRequest("POST", "/api/skillsets", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadSkillset_Overwrite(t *testing.T) {
	env, _ := newSkillsetTestEnv(t)

	w := uploadMultipart(t, env, validSkillsetYAML)
	if w.Code != http.StatusCreated {
		t.Fatalf("first upload: expected 201, got %d", w.Code)
	}

	updated := `name: test-skillset
version: "2.0.0"
contract_version: "2.0.0"
tools:
  - name: calculator
    type: builtin
  - name: web_search
    type: builtin
`

	w = uploadMultipart(t, env, updated)
	if w.Code != http.StatusCreated {
		t.Fatalf("second upload: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var sk skillsetSummary
	if err := json.NewDecoder(w.Body).Decode(&sk); err != nil {
		t.Fatal(err)
	}
	if sk.Version != "2.0.0" {
		t.Errorf("version = %q, want 2.0.0", sk.Version)
	}
	if sk.ToolCount != 2 {
		t.Errorf("tool_count = %d, want 2", sk.ToolCount)
	}
}

func TestSkillsetAuth_Required(t *testing.T) {
	env, _ := newSkillsetTestEnv(t)
	for _, path := range []string{"/api/skillsets", "/api/skillsets/test"} {
		w := env.doRequestNoAuth("GET", path)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("GET %s without auth: expected 401, got %d", path, w.Code)
		}
	}
}
