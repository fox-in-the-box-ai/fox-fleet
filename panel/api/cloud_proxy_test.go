package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func parseTestPort(t *testing.T, rawURL string) int {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse URL %q: %v", rawURL, err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse port from %q: %v", rawURL, err)
	}
	return port
}

func TestLegacyCloudRedirect_301ToAdmin(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "GET", "/cloud/something", "")
	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusMovedPermanently)
	}
	if loc := w.Header().Get("Location"); loc != "/admin/" {
		t.Errorf("Location = %q, want %q", loc, "/admin/")
	}
}

func TestLegacyCloudRedirect_301WithSessionCookie(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	cookie := extractSessionCookie(w)
	if cookie == nil {
		t.Fatal("no session cookie")
	}

	w = doCloudRequest(t, env, "GET", "/cloud/app", "", cookie)
	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusMovedPermanently)
	}
	if loc := w.Header().Get("Location"); loc != "/admin/" {
		t.Errorf("Location = %q, want %q", loc, "/admin/")
	}
}

func TestLegacyCloudRedirect_301OnNestedPath(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "GET", "/cloud/api/v1/models", "")
	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusMovedPermanently)
	}
	if loc := w.Header().Get("Location"); loc != "/admin/" {
		t.Errorf("Location = %q, want %q", loc, "/admin/")
	}
}

func TestCloudRoot_RedirectsToLogin(t *testing.T) {
	env := newTestEnvWithCloud(t)

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	env.server.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}
	if loc := w.Header().Get("Location"); loc != "/cloud/login" {
		t.Errorf("Location = %q, want %q", loc, "/cloud/login")
	}
}

func TestCloudRoot_RedirectsToAdminWhenAuthenticated(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	cookie := extractSessionCookie(w)

	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	env.server.Handler().ServeHTTP(rec, r)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin/" {
		t.Errorf("Location = %q, want %q", loc, "/admin/")
	}
}

func TestCloudLoginPage_ServesHTML(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "GET", "/cloud/login", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Fox Fleet") {
		t.Error("login page should contain Fox Fleet branding")
	}
	if !strings.Contains(body, "Sign In") {
		t.Error("login page should contain Sign In button")
	}
	if !strings.Contains(body, "/cloud/login") {
		t.Error("login page should post to /cloud/login")
	}
}

func TestCloudLoginPage_RedirectsWhenAuthenticated(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	cookie := extractSessionCookie(w)

	w = doCloudRequest(t, env, "GET", "/cloud/login", "", cookie)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}
	if loc := w.Header().Get("Location"); loc != "/admin/" {
		t.Errorf("Location = %q, want %q", loc, "/admin/")
	}
}

func TestCloudLoginPage_JSRedirectsToAdmin(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "GET", "/cloud/login", "")
	body := w.Body.String()
	if !strings.Contains(body, `window.location.href="/admin/"`) {
		t.Error("login page JS should redirect to /admin/ on success")
	}
}

func TestCloudLoginPage_HasSecurityHeaders(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "GET", "/cloud/login", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if v := w.Header().Get("X-Frame-Options"); v != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", v)
	}
	if v := w.Header().Get("X-Content-Type-Options"); v != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", v)
	}

	csp := w.Header().Get("Content-Security-Policy")
	for _, directive := range []string{"connect-src 'self'", "form-action 'self'"} {
		if !strings.Contains(csp, directive) {
			t.Errorf("CSP missing %q — login page fetch will be blocked by browsers.\nCSP: %s", directive, csp)
		}
	}
}
