package api

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/cloud"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/events"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
)

func newDispatchTestServer(t *testing.T, domain string) (*Server, *registry.Registry, *cloud.UserStore, *cloud.SessionStore) {
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
		EventLog:     events.NewLog(10),
		SigningKey:   testSigningKey,
		Logger:       slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		UserStore:    users,
		SessionStore: sessions,
		Cloud: CloudConfig{
			CookieName: "fox_cloud_session",
			Secure:     true,
			Domain:     domain,
			SessionTTL: time.Hour,
		},
	})

	return srv, reg, users, sessions
}

func TestHostDispatcher_BaseDomainRoutesToMux(t *testing.T) {
	srv, _, _, _ := newDispatchTestServer(t, "fleet.example.com")
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
	srv, _, _, _ := newDispatchTestServer(t, "fleet.example.com")
	handler := srv.Handler()

	req := httptest.NewRequest("GET", "/healthz", nil)
	req.Host = "fleet.example.com:9090"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("base domain with port /healthz: got %d, want 200", w.Code)
	}
}

func TestHostDispatcher_SubdomainRedirectsToLoginWithoutSession(t *testing.T) {
	srv, _, _, _ := newDispatchTestServer(t, "fleet.example.com")
	handler := srv.Handler()

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "alice.fleet.example.com"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("subdomain without session: got %d, want 303", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want /login", loc)
	}
}

func TestHostDispatcher_SubdomainProxiesToInstance(t *testing.T) {
	srv, reg, users, sessions := newDispatchTestServer(t, "fleet.example.com")

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello:%s", r.URL.Path)
	}))
	defer backend.Close()

	port := parseTestPort(t, backend.URL)
	if err := reg.Create(registry.Instance{
		ID: "fox-alice", ImageDigest: "sha256:abc", Port: port,
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

	token, _, err := sessions.Create("alice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/some/path", nil)
	req.Host = "alice.fleet.example.com"
	req.AddCookie(&http.Cookie{Name: "fox_cloud_session", Value: token})
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("subdomain proxy: got %d, want 200; body = %s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); body != "hello:/some/path" {
		t.Errorf("body = %q, want %q", body, "hello:/some/path")
	}
}

func TestHostDispatcher_SubdomainRejectsWrongUser(t *testing.T) {
	srv, _, users, sessions := newDispatchTestServer(t, "fleet.example.com")

	if _, err := users.Create("alice", "password123"); err != nil {
		t.Fatal(err)
	}
	if _, err := users.Create("bob", "password123"); err != nil {
		t.Fatal(err)
	}

	token, _, err := sessions.Create("bob", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "alice.fleet.example.com"
	req.AddCookie(&http.Cookie{Name: "fox_cloud_session", Value: token})
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("wrong user on subdomain: got %d, want 403", w.Code)
	}
}

func TestHostDispatcher_Subdomain503WhenNoInstance(t *testing.T) {
	srv, _, users, sessions := newDispatchTestServer(t, "fleet.example.com")

	if _, err := users.Create("alice", "password123"); err != nil {
		t.Fatal(err)
	}

	token, _, err := sessions.Create("alice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "alice.fleet.example.com"
	req.AddCookie(&http.Cookie{Name: "fox_cloud_session", Value: token})
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("no instance: got %d, want 503", w.Code)
	}
	if !strings.Contains(w.Body.String(), "no instance assigned") {
		t.Errorf("body should mention 'no instance assigned', got: %s", w.Body.String())
	}
}

func TestHostDispatcher_SubdomainInjectsXFoxAuth(t *testing.T) {
	srv, reg, users, sessions := newDispatchTestServer(t, "fleet.example.com")

	var gotHeader string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Fox-Auth")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	port := parseTestPort(t, backend.URL)
	if err := reg.Create(registry.Instance{
		ID: "fox-alice", ImageDigest: "sha256:abc", Port: port,
		DataDir: "/data/fox-alice", Status: "running",
		InstancePassword: "per-instance-secret",
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

	token, _, err := sessions.Create("alice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Host = "alice.fleet.example.com"
	req.AddCookie(&http.Cookie{Name: "fox_cloud_session", Value: token})
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("proxy: got %d, want 200", w.Code)
	}
	if gotHeader != "per-instance-secret" {
		t.Errorf("X-Fox-Auth = %q, want %q", gotHeader, "per-instance-secret")
	}
}

func TestHostDispatcher_SubdomainFallsBackToSharedPassword(t *testing.T) {
	srv, reg, users, sessions := newDispatchTestServer(t, "fleet.example.com")

	var gotHeader string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Fox-Auth")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	port := parseTestPort(t, backend.URL)
	if err := reg.Create(registry.Instance{
		ID: "fox-alice", ImageDigest: "sha256:abc", Port: port,
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

	token, _, err := sessions.Create("alice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Host = "alice.fleet.example.com"
	req.AddCookie(&http.Cookie{Name: "fox_cloud_session", Value: token})
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("proxy: got %d, want 200", w.Code)
	}
	if gotHeader != "test-pwd" {
		t.Errorf("X-Fox-Auth = %q, want shared fallback %q", gotHeader, "test-pwd")
	}
}

func TestHostDispatcher_NestedSubdomainRejected(t *testing.T) {
	srv, _, _, _ := newDispatchTestServer(t, "fleet.example.com")
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
	srv, _, _, _ := newDispatchTestServer(t, "fleet.example.com")
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
	srv, _, _, _ := newDispatchTestServer(t, "")
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

func TestSubdomain_DeleteUserInvalidatesAccess(t *testing.T) {
	srv, reg, users, sessions := newDispatchTestServer(t, "fleet.example.com")

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	port := parseTestPort(t, backend.URL)
	if err := reg.Create(registry.Instance{
		ID: "fox-alice", ImageDigest: "sha256:abc", Port: port,
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

	token, _, err := sessions.Create("alice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "alice.fleet.example.com"
	req.AddCookie(&http.Cookie{Name: "fox_cloud_session", Value: token})
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("before delete: got %d, want 200", w.Code)
	}

	if err := users.Delete("alice"); err != nil {
		t.Fatal(err)
	}

	req = httptest.NewRequest("GET", "/", nil)
	req.Host = "alice.fleet.example.com"
	req.AddCookie(&http.Cookie{Name: "fox_cloud_session", Value: token})
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatal("after user delete: should not proxy (got 200)")
	}
}

func TestSubdomain_MultiUserIsolation(t *testing.T) {
	srv, reg, users, sessions := newDispatchTestServer(t, "fleet.example.com")

	backendAlice := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ok")
	}))
	defer backendAlice.Close()
	backendBob := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ok")
	}))
	defer backendBob.Close()

	portAlice := parseTestPort(t, backendAlice.URL)
	portBob := parseTestPort(t, backendBob.URL)
	if err := reg.Create(registry.Instance{
		ID: "fox-alice", ImageDigest: "sha256:abc", Port: portAlice,
		DataDir: "/data/fox-alice", Status: "running",
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Create(registry.Instance{
		ID: "fox-bob", ImageDigest: "sha256:abc", Port: portBob,
		DataDir: "/data/fox-bob", Status: "running",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := users.Create("alice", "password123"); err != nil {
		t.Fatal(err)
	}
	if _, err := users.Create("bob", "password456"); err != nil {
		t.Fatal(err)
	}
	aliceInst := "fox-alice"
	if _, err := users.Update("alice", nil, &aliceInst); err != nil {
		t.Fatal(err)
	}
	bobInst := "fox-bob"
	if _, err := users.Update("bob", nil, &bobInst); err != nil {
		t.Fatal(err)
	}

	aliceToken, _, err := sessions.Create("alice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	bobToken, _, err := sessions.Create("bob", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "alice.fleet.example.com"
	req.AddCookie(&http.Cookie{Name: "fox_cloud_session", Value: aliceToken})
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("alice on alice.subdomain: got %d, want 200", w.Code)
	}

	req = httptest.NewRequest("GET", "/", nil)
	req.Host = "bob.fleet.example.com"
	req.AddCookie(&http.Cookie{Name: "fox_cloud_session", Value: aliceToken})
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("alice on bob.subdomain: got %d, want 403", w.Code)
	}

	req = httptest.NewRequest("GET", "/", nil)
	req.Host = "alice.fleet.example.com"
	req.AddCookie(&http.Cookie{Name: "fox_cloud_session", Value: bobToken})
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("bob on alice.subdomain: got %d, want 403", w.Code)
	}

	req = httptest.NewRequest("GET", "/", nil)
	req.Host = "bob.fleet.example.com"
	req.AddCookie(&http.Cookie{Name: "fox_cloud_session", Value: bobToken})
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("bob on bob.subdomain: got %d, want 200", w.Code)
	}
}

func TestSubdomain_503WhenBackendUnreachable(t *testing.T) {
	srv, reg, users, sessions := newDispatchTestServer(t, "fleet.example.com")

	if err := reg.Create(registry.Instance{
		ID: "fox-alice", ImageDigest: "sha256:abc", Port: 59999,
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

	token, _, err := sessions.Create("alice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "alice.fleet.example.com"
	req.AddCookie(&http.Cookie{Name: "fox_cloud_session", Value: token})
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("unreachable backend: got %d, want 503", w.Code)
	}
	if !strings.Contains(w.Body.String(), "unreachable") {
		t.Errorf("body should mention unreachable, got: %s", w.Body.String())
	}
}

func TestSubdomain_PreservesQueryString(t *testing.T) {
	srv, reg, users, sessions := newDispatchTestServer(t, "fleet.example.com")

	var gotQuery string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	port := parseTestPort(t, backend.URL)
	if err := reg.Create(registry.Instance{
		ID: "fox-alice", ImageDigest: "sha256:abc", Port: port,
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

	token, _, err := sessions.Create("alice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/models?format=json&page=2", nil)
	req.Host = "alice.fleet.example.com"
	req.AddCookie(&http.Cookie{Name: "fox_cloud_session", Value: token})
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("proxy with query: got %d, want 200", w.Code)
	}
	if gotQuery != "format=json&page=2" {
		t.Errorf("query = %q, want %q", gotQuery, "format=json&page=2")
	}
}

func TestSubdomain_ForwardsPOSTBody(t *testing.T) {
	srv, reg, users, sessions := newDispatchTestServer(t, "fleet.example.com")

	var gotMethod, gotBody string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	port := parseTestPort(t, backend.URL)
	if err := reg.Create(registry.Instance{
		ID: "fox-alice", ImageDigest: "sha256:abc", Port: port,
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

	token, _, err := sessions.Create("alice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	postBody := `{"model":"hermes","prompt":"hello"}`
	req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(postBody))
	req.Host = "alice.fleet.example.com"
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "fox_cloud_session", Value: token})
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST proxy: got %d, want 200", w.Code)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotBody != postBody {
		t.Errorf("body = %q, want %q", gotBody, postBody)
	}
}

func TestSubdomain_503HasSecurityHeaders(t *testing.T) {
	srv, _, users, sessions := newDispatchTestServer(t, "fleet.example.com")

	if _, err := users.Create("alice", "password123"); err != nil {
		t.Fatal(err)
	}

	token, _, err := sessions.Create("alice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "alice.fleet.example.com"
	req.AddCookie(&http.Cookie{Name: "fox_cloud_session", Value: token})
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("503 page: got %d, want 503", w.Code)
	}

	if v := w.Header().Get("X-Frame-Options"); v != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", v)
	}
	if v := w.Header().Get("X-Content-Type-Options"); v != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", v)
	}
	if v := w.Header().Get("Referrer-Policy"); v != "strict-origin-when-cross-origin" {
		t.Errorf("Referrer-Policy = %q, want strict-origin-when-cross-origin", v)
	}
	csp := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'none'") {
		t.Errorf("CSP missing default-src 'none': %s", csp)
	}
}

func TestSubdomain_AuthenticatedLoginProxiesToFox(t *testing.T) {
	srv, reg, users, sessions := newDispatchTestServer(t, "fleet.example.com")

	var gotPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprintf(w, "fox-login-page")
	}))
	defer backend.Close()

	port := parseTestPort(t, backend.URL)
	if err := reg.Create(registry.Instance{
		ID: "fox-alice", ImageDigest: "sha256:abc", Port: port,
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

	token, _, err := sessions.Create("alice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/login", nil)
	req.Host = "alice.fleet.example.com"
	req.AddCookie(&http.Cookie{Name: "fox_cloud_session", Value: token})
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("authenticated /login: got %d, want 200 (proxied to Fox)", w.Code)
	}
	if gotPath != "/login" {
		t.Errorf("proxied path = %q, want /login", gotPath)
	}
	if body := w.Body.String(); body != "fox-login-page" {
		t.Errorf("body = %q, want fox-login-page", body)
	}
}

func TestSubdomain_AuthenticatedLoginPostProxiesToFox(t *testing.T) {
	srv, reg, users, sessions := newDispatchTestServer(t, "fleet.example.com")

	var gotMethod, gotBody string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	port := parseTestPort(t, backend.URL)
	if err := reg.Create(registry.Instance{
		ID: "fox-alice", ImageDigest: "sha256:abc", Port: port,
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

	token, _, err := sessions.Create("alice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	postBody := "password=secret123"
	req := httptest.NewRequest("POST", "/login", strings.NewReader(postBody))
	req.Host = "alice.fleet.example.com"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "fox_cloud_session", Value: token})
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("authenticated POST /login: got %d, want 200 (proxied to Fox)", w.Code)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotBody != postBody {
		t.Errorf("body = %q, want %q", gotBody, postBody)
	}
}

func TestSubdomain_UnauthenticatedLoginShowsFleetPage(t *testing.T) {
	srv, _, _, _ := newDispatchTestServer(t, "fleet.example.com")

	req := httptest.NewRequest("GET", "/login", nil)
	req.Host = "alice.fleet.example.com"
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unauthenticated /login: got %d, want 200 (Fleet login page)", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	if !strings.Contains(w.Body.String(), "Fox Fleet") {
		t.Error("body should contain Fleet login page content")
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
		{"[::1]:9090", "[::1]"},
		{"[::1]", "[::1]"},
		{"[2001:db8::1]:443", "[2001:db8::1]"},
	}
	for _, tt := range tests {
		got := stripPort(tt.input)
		if got != tt.want {
			t.Errorf("stripPort(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
