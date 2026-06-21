package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
)

func subdomainRequest(t *testing.T, srv *Server, method, slug, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reader)
	req.Host = slug + ".fleet.example.com"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cookies {
		if c != nil {
			req.AddCookie(c)
		}
	}
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	return w
}

func TestSubdomainLogin_Success(t *testing.T) {
	srv, _, users, _ := newDispatchTestServer(t, "fleet.example.com")

	if _, err := users.Create("alice", "password123"); err != nil {
		t.Fatal(err)
	}

	w := subdomainRequest(t, srv, "POST", "alice", "/login", `{"username":"alice","password":"password123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login: got %d, want 200; body = %s", w.Code, w.Body.String())
	}

	cookie := extractSessionCookie(w)
	if cookie == nil {
		t.Fatal("no session cookie set")
	}
}

func TestSubdomainLogin_WrongPassword(t *testing.T) {
	srv, _, users, _ := newDispatchTestServer(t, "fleet.example.com")

	if _, err := users.Create("alice", "password123"); err != nil {
		t.Fatal(err)
	}

	w := subdomainRequest(t, srv, "POST", "alice", "/login", `{"username":"alice","password":"wrong"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: got %d, want 401", w.Code)
	}
}

func TestSubdomainLogin_UsernameMismatch(t *testing.T) {
	srv, _, users, _ := newDispatchTestServer(t, "fleet.example.com")

	if _, err := users.Create("alice", "password123"); err != nil {
		t.Fatal(err)
	}
	if _, err := users.Create("bob", "password123"); err != nil {
		t.Fatal(err)
	}

	w := subdomainRequest(t, srv, "POST", "alice", "/login", `{"username":"bob","password":"password123"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("username mismatch: got %d, want 403", w.Code)
	}
}

func TestSubdomainLogin_MissingFields(t *testing.T) {
	srv, _, _, _ := newDispatchTestServer(t, "fleet.example.com")

	w := subdomainRequest(t, srv, "POST", "alice", "/login", `{"username":"alice"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing password: got %d, want 400", w.Code)
	}
}

func TestSubdomainLogin_PasswordOnly(t *testing.T) {
	srv, _, users, _ := newDispatchTestServer(t, "fleet.example.com")

	if _, err := users.Create("alice", "password123"); err != nil {
		t.Fatal(err)
	}

	w := subdomainRequest(t, srv, "POST", "alice", "/login", `{"password":"password123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("password-only login: got %d, want 200; body = %s", w.Code, w.Body.String())
	}

	cookie := extractSessionCookie(w)
	if cookie == nil {
		t.Fatal("no session cookie set on password-only login")
	}
}

func TestSubdomainLogin_PasswordOnly_WrongPassword(t *testing.T) {
	srv, _, users, _ := newDispatchTestServer(t, "fleet.example.com")

	if _, err := users.Create("alice", "password123"); err != nil {
		t.Fatal(err)
	}

	w := subdomainRequest(t, srv, "POST", "alice", "/login", `{"password":"wrong"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: got %d, want 401", w.Code)
	}
}

func TestSubdomainLogin_PasswordOnly_MissingPassword(t *testing.T) {
	srv, _, _, _ := newDispatchTestServer(t, "fleet.example.com")

	w := subdomainRequest(t, srv, "POST", "alice", "/login", `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing password: got %d, want 400", w.Code)
	}
}

func TestSubdomainLoginPage_ServesHTML(t *testing.T) {
	srv, _, _, _ := newDispatchTestServer(t, "fleet.example.com")

	w := subdomainRequest(t, srv, "GET", "alice", "/login", "")
	if w.Code != http.StatusOK {
		t.Fatalf("login page: got %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Fox Fleet") {
		t.Error("login page should contain Fox Fleet branding")
	}
	if !strings.Contains(body, `fetch("/login"`) {
		t.Error("login page should POST to /login")
	}
	if !strings.Contains(body, `window.location.href="/"`) {
		t.Error("login page should redirect to / on success")
	}
	if strings.Contains(body, `<label for="username"`) {
		t.Error("subdomain login page should NOT have a username label (password-only)")
	}
	if !strings.Contains(body, `type="hidden" id="username"`) {
		t.Error("subdomain login page should have a hidden username field for autocomplete")
	}
	if !strings.Contains(body, `id="subtitle"`) {
		t.Error("subdomain login page should have a subtitle element for slug display")
	}
	if !strings.Contains(body, `rel="icon"`) {
		t.Error("subdomain login page should have a favicon link tag")
	}
}

func TestSubdomainLoginPage_HasFavicon(t *testing.T) {
	if !strings.Contains(subdomainLoginPage, `rel="icon" type="image/svg+xml"`) {
		t.Error("subdomain login page missing SVG favicon link")
	}
}

func TestSubdomainLoginPage_AuthenticatedProxiesToFox(t *testing.T) {
	srv, _, users, sessions := newDispatchTestServer(t, "fleet.example.com")

	if _, err := users.Create("alice", "password123"); err != nil {
		t.Fatal(err)
	}
	token, _, err := sessions.Create("alice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	cookie := &http.Cookie{Name: "fox_cloud_session", Value: token}
	w := subdomainRequest(t, srv, "GET", "alice", "/login", "", cookie)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("authenticated /login without instance: got %d, want 503 (proxied to Fox, not Fleet redirect)", w.Code)
	}
}

func TestSubdomainLoginPage_WrongUserSessionRejects(t *testing.T) {
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

	cookie := &http.Cookie{Name: "fox_cloud_session", Value: token}
	w := subdomainRequest(t, srv, "GET", "alice", "/login", "", cookie)
	if w.Code != http.StatusForbidden {
		t.Fatalf("wrong user on /login: got %d, want 403", w.Code)
	}
}

func TestSubdomainLoginPage_HasSecurityHeaders(t *testing.T) {
	srv, _, _, _ := newDispatchTestServer(t, "fleet.example.com")

	w := subdomainRequest(t, srv, "GET", "alice", "/login", "")
	if v := w.Header().Get("X-Frame-Options"); v != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", v)
	}
	if v := w.Header().Get("X-Content-Type-Options"); v != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", v)
	}
	csp := w.Header().Get("Content-Security-Policy")
	for _, directive := range []string{"connect-src 'self'", "form-action 'self'"} {
		if !strings.Contains(csp, directive) {
			t.Errorf("CSP missing %q\nCSP: %s", directive, csp)
		}
	}
}

func TestSubdomainLogout(t *testing.T) {
	srv, _, users, sessions := newDispatchTestServer(t, "fleet.example.com")

	if _, err := users.Create("alice", "password123"); err != nil {
		t.Fatal(err)
	}
	token, _, err := sessions.Create("alice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	cookie := &http.Cookie{Name: "fox_cloud_session", Value: token}
	w := subdomainRequest(t, srv, "POST", "alice", "/logout", "", cookie)
	if w.Code != http.StatusNoContent {
		t.Fatalf("logout: got %d, want 204", w.Code)
	}

	w = subdomainRequest(t, srv, "GET", "alice", "/", "", cookie)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("post-logout: got %d, want 303 redirect to /login", w.Code)
	}
}

func TestSubdomainLogin_FullE2E(t *testing.T) {
	srv, reg, users, _ := newDispatchTestServer(t, "fleet.example.com")

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello from instance")
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

	w := subdomainRequest(t, srv, "POST", "alice", "/login", `{"username":"alice","password":"password123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login: got %d; body = %s", w.Code, w.Body.String())
	}
	cookie := extractSessionCookie(w)
	if cookie == nil {
		t.Fatal("no session cookie")
	}

	w = subdomainRequest(t, srv, "GET", "alice", "/", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("proxy: got %d, want 200; body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "hello from instance") {
		t.Errorf("body = %q, want 'hello from instance'", w.Body.String())
	}

	w = subdomainRequest(t, srv, "POST", "alice", "/logout", "", cookie)
	if w.Code != http.StatusNoContent {
		t.Fatalf("logout: got %d", w.Code)
	}

	w = subdomainRequest(t, srv, "GET", "alice", "/", "", cookie)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("post-logout: got %d, want 303", w.Code)
	}
}
