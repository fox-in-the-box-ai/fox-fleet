package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/provisioner"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
)

const testSecret = "test-admin-secret-9f4a"

// --- compile-time interface checks ---

var (
	_ provisioner.Provisioner    = (*fakeProvisioner)(nil)
	_ plugins.DeploymentPlugin   = (*fakePlugin)(nil)
)

// --- fakes ---

type fakeProvisioner struct {
	mu          sync.Mutex
	provisions  []provisioner.Request
	provisionCh chan struct{}
	destroyFn   func(string, bool) error
}

func (f *fakeProvisioner) Provision(_ context.Context, req provisioner.Request) (*provisioner.Instance, error) {
	f.mu.Lock()
	f.provisions = append(f.provisions, req)
	f.mu.Unlock()
	if f.provisionCh != nil {
		f.provisionCh <- struct{}{}
	}
	return &provisioner.Instance{
		ID:     req.InstanceID,
		Port:   9100,
		Status: "running",
	}, nil
}

func (f *fakeProvisioner) Destroy(_ context.Context, id string, removeData bool) error {
	if f.destroyFn != nil {
		return f.destroyFn(id, removeData)
	}
	return nil
}

type fakePlugin struct{}

func (f *fakePlugin) Provision(_ context.Context, _ plugins.ProvisionRequest) error { return nil }
func (f *fakePlugin) HealthCheck(_ context.Context, _ string) (plugins.HealthStatus, error) {
	return plugins.HealthStatus{Healthy: true, Ready: true, CheckedAt: time.Now().UTC()}, nil
}
func (f *fakePlugin) Configure(_ context.Context, _ string, _ plugins.InstanceConfig) error {
	return nil
}
func (f *fakePlugin) Rollout(_ context.Context, _ string, _ plugins.ImageRef) error  { return nil }
func (f *fakePlugin) Rollback(_ context.Context, _ string, _ plugins.ImageRef) error { return nil }
func (f *fakePlugin) Destroy(_ context.Context, _ string) error                      { return nil }
func (f *fakePlugin) Logs(_ context.Context, _ string, _ plugins.LogOpts) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("log line 1\nlog line 2\n")), nil
}

// --- helpers ---

type testEnv struct {
	server   *Server
	registry *registry.Registry
	prov     *fakeProvisioner
	logBuf   *bytes.Buffer
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	dir := t.TempDir()
	reg, err := registry.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Close() })

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
	})

	return &testEnv{
		server:   srv,
		registry: reg,
		prov:     prov,
		logBuf:   &logBuf,
	}
}

func (e *testEnv) doRequest(method, path string, body string) *httptest.ResponseRecorder {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Authorization", "Bearer "+testSecret)
	w := httptest.NewRecorder()
	e.server.Handler().ServeHTTP(w, req)
	return w
}

func (e *testEnv) doRequestNoAuth(method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	e.server.Handler().ServeHTTP(w, req)
	return w
}

func (e *testEnv) seedInstance(t *testing.T, id string, port int) {
	t.Helper()
	err := e.registry.Create(registry.Instance{
		ID:          id,
		ImageDigest: "sha256:abc123",
		Port:        port,
		DataDir:     "/data/" + id,
		Status:      "running",
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatal(err)
	}
}

// --- auth tests ---

func TestAuthMissing(t *testing.T) {
	env := newTestEnv(t)
	w := env.doRequestNoAuth("GET", "/api/instances")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	var resp apiError
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != "unauthorized" {
		t.Fatalf("expected error code 'unauthorized', got %q", resp.Error)
	}
}

func TestAuthInvalidToken(t *testing.T) {
	env := newTestEnv(t)
	req := httptest.NewRequest("GET", "/api/instances", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	env.server.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthWrongScheme(t *testing.T) {
	env := newTestEnv(t)
	req := httptest.NewRequest("GET", "/api/instances", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()
	env.server.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthValidToken(t *testing.T) {
	env := newTestEnv(t)
	w := env.doRequest("GET", "/api/instances", "")
	if w.Code == http.StatusUnauthorized {
		t.Fatal("expected non-401 with valid token")
	}
}

func TestSecretNotInLogs(t *testing.T) {
	env := newTestEnv(t)

	req := httptest.NewRequest("GET", "/api/instances", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	env.server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	logOutput := env.logBuf.String()
	if strings.Contains(logOutput, testSecret) {
		t.Fatal("admin secret found in log output after unauthorized request")
	}
	if strings.Contains(w.Body.String(), testSecret) {
		t.Fatal("admin secret found in response body")
	}
}

// --- list tests ---

func TestListEmpty(t *testing.T) {
	env := newTestEnv(t)
	w := env.doRequest("GET", "/api/instances", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var items []instanceItem
	json.NewDecoder(w.Body).Decode(&items)
	if len(items) != 0 {
		t.Fatalf("expected empty list, got %d items", len(items))
	}
}

func TestListWithInstances(t *testing.T) {
	env := newTestEnv(t)
	env.seedInstance(t, "fox-1", 9100)
	env.seedInstance(t, "fox-2", 9101)

	w := env.doRequest("GET", "/api/instances", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var items []instanceItem
	json.NewDecoder(w.Body).Decode(&items)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != "fox-1" {
		t.Fatalf("expected first item ID 'fox-1', got %q", items[0].ID)
	}
}

func TestListIncludesHealth(t *testing.T) {
	env := newTestEnv(t)
	env.seedInstance(t, "fox-1", 9100)

	env.server.poller.poll(context.Background())

	w := env.doRequest("GET", "/api/instances", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var items []instanceItem
	json.NewDecoder(w.Body).Decode(&items)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Health == nil {
		t.Fatal("expected health field to be populated after poll")
	}
	if !items[0].Health.Healthy {
		t.Fatal("expected healthy=true")
	}
}

// --- detail tests ---

func TestDetailFound(t *testing.T) {
	env := newTestEnv(t)
	env.seedInstance(t, "fox-1", 9100)

	w := env.doRequest("GET", "/api/instances/fox-1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var detail instanceDetail
	json.NewDecoder(w.Body).Decode(&detail)
	if detail.ID != "fox-1" {
		t.Fatalf("expected id 'fox-1', got %q", detail.ID)
	}
	if detail.Port != 9100 {
		t.Fatalf("expected port 9100, got %d", detail.Port)
	}
	if detail.Logs == "" {
		t.Fatal("expected non-empty logs from fake plugin")
	}
}

func TestDetailNotFound(t *testing.T) {
	env := newTestEnv(t)
	w := env.doRequest("GET", "/api/instances/nonexistent", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	var resp apiError
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != "not_found" {
		t.Fatalf("expected error code 'not_found', got %q", resp.Error)
	}
}

// --- create tests ---

func TestCreateSuccess(t *testing.T) {
	env := newTestEnv(t)
	w := env.doRequest("POST", "/api/instances", `{"id":"fox-1"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp createResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.ID != "fox-1" {
		t.Fatalf("expected id 'fox-1', got %q", resp.ID)
	}
	if resp.Status != "provisioning" {
		t.Fatalf("expected status 'provisioning', got %q", resp.Status)
	}

	select {
	case <-env.prov.provisionCh:
	case <-time.After(2 * time.Second):
		t.Fatal("provisioner not called within timeout")
	}

	env.prov.mu.Lock()
	defer env.prov.mu.Unlock()
	if len(env.prov.provisions) != 1 {
		t.Fatalf("expected 1 provision call, got %d", len(env.prov.provisions))
	}
	if env.prov.provisions[0].InstanceID != "fox-1" {
		t.Fatalf("expected instance ID 'fox-1', got %q", env.prov.provisions[0].InstanceID)
	}
}

func TestCreateConflict(t *testing.T) {
	env := newTestEnv(t)
	env.seedInstance(t, "fox-1", 9100)

	w := env.doRequest("POST", "/api/instances", `{"id":"fox-1"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
	var resp apiError
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != "conflict" {
		t.Fatalf("expected error code 'conflict', got %q", resp.Error)
	}
}

func TestCreateCapReached(t *testing.T) {
	env := newTestEnv(t)
	env.seedInstance(t, "fox-1", 9100)
	env.seedInstance(t, "fox-2", 9101)

	w := env.doRequest("POST", "/api/instances", `{"id":"fox-3"}`)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	var resp apiError
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != "cap_reached" {
		t.Fatalf("expected error code 'cap_reached', got %q", resp.Error)
	}
}

func TestCreateMissingID(t *testing.T) {
	env := newTestEnv(t)
	w := env.doRequest("POST", "/api/instances", `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateInvalidJSON(t *testing.T) {
	env := newTestEnv(t)
	w := env.doRequest("POST", "/api/instances", `not json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateInvalidID(t *testing.T) {
	env := newTestEnv(t)
	w := env.doRequest("POST", "/api/instances", `{"id":"../../etc"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for path traversal ID, got %d", w.Code)
	}
}

// --- destroy tests ---

func TestDestroySuccess(t *testing.T) {
	env := newTestEnv(t)
	w := env.doRequest("DELETE", "/api/instances/fox-1", "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDestroyNotFound(t *testing.T) {
	env := newTestEnv(t)
	env.prov.destroyFn = func(_ string, _ bool) error {
		return fmt.Errorf("provisioner: %w", registry.ErrNotFound)
	}
	w := env.doRequest("DELETE", "/api/instances/nonexistent", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDestroyWithRemoveData(t *testing.T) {
	env := newTestEnv(t)
	var gotRemoveData bool
	env.prov.destroyFn = func(_ string, removeData bool) error {
		gotRemoveData = removeData
		return nil
	}
	w := env.doRequest("DELETE", "/api/instances/fox-1?remove_data=true", "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if !gotRemoveData {
		t.Fatal("expected remove_data=true to be passed to provisioner")
	}
}

// --- poller tests ---

func TestPollerGet(t *testing.T) {
	env := newTestEnv(t)
	env.seedInstance(t, "fox-1", 9100)

	env.server.poller.poll(context.Background())

	hs, ok := env.server.poller.Get("fox-1")
	if !ok {
		t.Fatal("expected health status for fox-1")
	}
	if !hs.Healthy {
		t.Fatal("expected healthy=true")
	}
	if !hs.Ready {
		t.Fatal("expected ready=true")
	}
}

func TestPollerMiss(t *testing.T) {
	env := newTestEnv(t)
	_, ok := env.server.poller.Get("nonexistent")
	if ok {
		t.Fatal("expected no health status for nonexistent instance")
	}
}
