package api

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/cloud"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
)

func newTLSCheckTestEnv(t *testing.T) (*Server, *registry.Registry, *cloud.UserStore) {
	t.Helper()
	dir := t.TempDir()
	reg, err := registry.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Close() })

	db := reg.DB()
	users := cloud.NewUserStore(db)
	sessions := cloud.NewSessionStore(db)

	srv := NewServer(Deps{
		Registry:     reg,
		Provisioner:  &fakeProvisioner{provisionCh: make(chan struct{}, 1)},
		Plugin:       &fakePlugin{},
		AdminSecret:  testSecret,
		InstancePwd:  "test-pwd",
		MaxInstances: 2,
		SigningKey:    testSigningKey,
		Logger:       slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		UserStore:    users,
		SessionStore: sessions,
		Cloud: CloudConfig{
			CookieName: "fox_cloud_session",
			Domain:     "fleet.example.com",
			SessionTTL: time.Hour,
		},
	})

	return srv, reg, users
}

func TestTLSCheck_MissingDomain(t *testing.T) {
	srv, _, _ := newTLSCheckTestEnv(t)
	req := httptest.NewRequest("GET", "/cloud/tls-check", nil)
	req.Host = "fleet.example.com"
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("missing domain param: got %d, want 403", w.Code)
	}
}

func TestTLSCheck_WrongBaseDomain(t *testing.T) {
	srv, _, _ := newTLSCheckTestEnv(t)
	req := httptest.NewRequest("GET", "/cloud/tls-check?domain=alice.other.com", nil)
	req.Host = "fleet.example.com"
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("wrong base domain: got %d, want 403", w.Code)
	}
}

func TestTLSCheck_NestedSubdomain(t *testing.T) {
	srv, _, _ := newTLSCheckTestEnv(t)
	req := httptest.NewRequest("GET", "/cloud/tls-check?domain=a.b.fleet.example.com", nil)
	req.Host = "fleet.example.com"
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("nested subdomain: got %d, want 403", w.Code)
	}
}

func TestTLSCheck_UnknownUser(t *testing.T) {
	srv, _, _ := newTLSCheckTestEnv(t)
	req := httptest.NewRequest("GET", "/cloud/tls-check?domain=nobody.fleet.example.com", nil)
	req.Host = "fleet.example.com"
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("unknown user: got %d, want 403", w.Code)
	}
}

func TestTLSCheck_UserWithoutInstance(t *testing.T) {
	srv, _, users := newTLSCheckTestEnv(t)
	if _, err := users.Create("bob", "password123"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/cloud/tls-check?domain=bob.fleet.example.com", nil)
	req.Host = "fleet.example.com"
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("user without instance: got %d, want 403", w.Code)
	}
}

func TestTLSCheck_ValidSubdomain(t *testing.T) {
	srv, reg, users := newTLSCheckTestEnv(t)

	if err := reg.Create(registry.Instance{
		ID: "fox-alice", ImageDigest: "sha256:abc", Port: 9100,
		DataDir: "/data/fox-alice", Status: "running",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := users.Create("alice", "password123"); err != nil {
		t.Fatal(err)
	}
	instID := "fox-alice"
	if _, err := users.Update("alice", nil, &instID); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/cloud/tls-check?domain=alice.fleet.example.com", nil)
	req.Host = "fleet.example.com"
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("valid subdomain: got %d, want 200", w.Code)
	}
}
