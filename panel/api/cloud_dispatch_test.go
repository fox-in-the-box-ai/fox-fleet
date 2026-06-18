package api

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/events"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
)

func newDispatchTestServer(t *testing.T, domain string) *Server {
	t.Helper()
	dir := t.TempDir()
	reg, err := registry.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Close() })

	return NewServer(Deps{
		Registry:     reg,
		Provisioner:  &fakeProvisioner{provisionCh: make(chan struct{}, 1)},
		Plugin:       &fakePlugin{},
		AdminSecret:  testSecret,
		InstancePwd:  "test-pwd",
		MaxInstances: 2,
		EventLog:     events.NewLog(10),
		SigningKey:    testSigningKey,
		Logger:       slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		Cloud: CloudConfig{
			CookieName: "fox_cloud_session",
			Domain:     domain,
			SessionTTL: 3600,
		},
	})
}

func TestHostDispatcher_BaseDomainRoutesToMux(t *testing.T) {
	srv := newDispatchTestServer(t, "fleet.example.com")
	handler := srv.Handler()

	req := httptest.NewRequest("GET", "/healthz", nil)
	req.Host = "fleet.example.com"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("base domain /healthz: got %d, want 200", w.Code)
	}
}

func TestHostDispatcher_BaseDomainWithPort(t *testing.T) {
	srv := newDispatchTestServer(t, "fleet.example.com")
	handler := srv.Handler()

	req := httptest.NewRequest("GET", "/healthz", nil)
	req.Host = "fleet.example.com:9090"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("base domain with port /healthz: got %d, want 200", w.Code)
	}
}

func TestHostDispatcher_SubdomainRoutesToSubdomainHandler(t *testing.T) {
	srv := newDispatchTestServer(t, "fleet.example.com")
	handler := srv.Handler()

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "alice.fleet.example.com"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("subdomain request: got %d, want 503 (stub)", w.Code)
	}
}

func TestHostDispatcher_NestedSubdomainRejected(t *testing.T) {
	srv := newDispatchTestServer(t, "fleet.example.com")
	handler := srv.Handler()

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "a.b.fleet.example.com"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("nested subdomain: got %d, want 400", w.Code)
	}
}

func TestHostDispatcher_UnknownHostRoutesToMux(t *testing.T) {
	srv := newDispatchTestServer(t, "fleet.example.com")
	handler := srv.Handler()

	req := httptest.NewRequest("GET", "/healthz", nil)
	req.Host = "other.example.com"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unknown host /healthz: got %d, want 200 (routes to mux)", w.Code)
	}
}

func TestHostDispatcher_NoDomainReturnsRawMux(t *testing.T) {
	srv := newDispatchTestServer(t, "")
	srv.cloudCfg.Domain = ""
	handler := srv.Handler()

	req := httptest.NewRequest("GET", "/healthz", nil)
	req.Host = "anything.example.com"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("no domain configured, /healthz: got %d, want 200 (raw mux)", w.Code)
	}
}

func TestStripPort(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"fleet.example.com", "fleet.example.com"},
		{"fleet.example.com:9090", "fleet.example.com"},
		{"alice.fleet.example.com:443", "alice.fleet.example.com"},
		{"localhost:8080", "localhost"},
		{"localhost", "localhost"},
	}
	for _, tt := range tests {
		got := stripPort(tt.input)
		if got != tt.want {
			t.Errorf("stripPort(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
