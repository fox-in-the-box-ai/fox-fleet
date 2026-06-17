package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestCloudProxy_RedirectsUnauthenticated(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "GET", "/cloud/something", "")
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}
	if loc := w.Header().Get("Location"); loc != "/cloud/login" {
		t.Errorf("Location = %q, want %q", loc, "/cloud/login")
	}
}

func TestCloudProxy_RedirectsExpiredSession(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "GET", "/cloud/something", "",
		&http.Cookie{Name: "fox_cloud_session", Value: "bogus-token"})
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}
}

func TestCloudProxy_503WhenNoInstance(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	cookie := extractSessionCookie(w)
	if cookie == nil {
		t.Fatal("no session cookie")
	}

	w = doCloudRequest(t, env, "GET", "/cloud/app", "", cookie)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusServiceUnavailable, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "no instance assigned") {
		t.Errorf("body should mention 'no instance assigned', got: %s", w.Body.String())
	}
}

func TestCloudProxy_503WhenInstanceNotRunning(t *testing.T) {
	env := newTestEnvWithCloud(t)

	env.seedInstance(t, "stopped-inst", 19999)
	if err := env.registry.UpdateStatus("stopped-inst", "stopped"); err != nil {
		t.Fatal(err)
	}

	instID := "stopped-inst"
	if _, err := env.server.users.Update("alice", nil, &instID); err != nil {
		t.Fatalf("assign instance: %v", err)
	}

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	cookie := extractSessionCookie(w)

	w = doCloudRequest(t, env, "GET", "/cloud/app", "", cookie)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(w.Body.String(), "stopped") {
		t.Errorf("body should mention 'stopped', got: %s", w.Body.String())
	}
}

func TestCloudProxy_ProxiesToRunningInstance(t *testing.T) {
	env := newTestEnvWithCloud(t)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "proxied:%s", r.URL.Path)
	}))
	defer backend.Close()

	port := parseTestPort(t, backend.URL)
	env.seedInstance(t, "proxy-inst", port)

	instID := "proxy-inst"
	if _, err := env.server.users.Update("alice", nil, &instID); err != nil {
		t.Fatalf("assign instance: %v", err)
	}

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	cookie := extractSessionCookie(w)

	w = doCloudRequest(t, env, "GET", "/cloud/some/path", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}
	if body := w.Body.String(); body != "proxied:/some/path" {
		t.Errorf("body = %q, want %q", body, "proxied:/some/path")
	}
}

func TestCloudProxy_StripsCloudPrefix(t *testing.T) {
	env := newTestEnvWithCloud(t)

	var gotPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	port := parseTestPort(t, backend.URL)
	env.seedInstance(t, "strip-inst", port)

	instID := "strip-inst"
	if _, err := env.server.users.Update("alice", nil, &instID); err != nil {
		t.Fatal(err)
	}

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	cookie := extractSessionCookie(w)

	doCloudRequest(t, env, "GET", "/cloud/api/v1/models", "", cookie)
	if gotPath != "/api/v1/models" {
		t.Errorf("proxied path = %q, want %q", gotPath, "/api/v1/models")
	}
}

func TestCloudProxy_InjectsXFoxAuth(t *testing.T) {
	env := newTestEnvWithCloud(t)

	var gotHeader string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Fox-Auth")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	port := parseTestPort(t, backend.URL)
	env.seedInstance(t, "auth-inst", port)

	instID := "auth-inst"
	if _, err := env.server.users.Update("alice", nil, &instID); err != nil {
		t.Fatal(err)
	}

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	cookie := extractSessionCookie(w)

	doCloudRequest(t, env, "GET", "/cloud/test", "", cookie)
	if gotHeader != "test-instance-pwd" {
		t.Errorf("X-Fox-Auth = %q, want %q", gotHeader, "test-instance-pwd")
	}
}

func TestCloudProxy_RootRedirectsToLogin(t *testing.T) {
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

func TestCloudProxy_RootRedirectsToCloudWhenAuthenticated(t *testing.T) {
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
	if loc := rec.Header().Get("Location"); loc != "/cloud/" {
		t.Errorf("Location = %q, want %q", loc, "/cloud/")
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
	if loc := w.Header().Get("Location"); loc != "/cloud/" {
		t.Errorf("Location = %q, want %q", loc, "/cloud/")
	}
}

func TestCloudProxy_PreservesQueryString(t *testing.T) {
	env := newTestEnvWithCloud(t)

	var gotQuery string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	port := parseTestPort(t, backend.URL)
	env.seedInstance(t, "qs-inst", port)

	instID := "qs-inst"
	if _, err := env.server.users.Update("alice", nil, &instID); err != nil {
		t.Fatal(err)
	}

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	cookie := extractSessionCookie(w)

	doCloudRequest(t, env, "GET", "/cloud/api/v1/models?key=value&foo=bar", "", cookie)
	if gotQuery != "key=value&foo=bar" {
		t.Errorf("query = %q, want %q", gotQuery, "key=value&foo=bar")
	}
}

func TestCloudProxy_ForwardsPOSTBody(t *testing.T) {
	env := newTestEnvWithCloud(t)

	var gotMethod, gotBody string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	port := parseTestPort(t, backend.URL)
	env.seedInstance(t, "post-inst", port)

	instID := "post-inst"
	if _, err := env.server.users.Update("alice", nil, &instID); err != nil {
		t.Fatal(err)
	}

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	cookie := extractSessionCookie(w)

	doCloudRequest(t, env, "POST", "/cloud/api/chat", `{"message":"hello"}`, cookie)
	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotBody != `{"message":"hello"}` {
		t.Errorf("body = %q, want %q", gotBody, `{"message":"hello"}`)
	}
}

func TestCloudProxy_503WhenBackendUnreachable(t *testing.T) {
	env := newTestEnvWithCloud(t)

	env.seedInstance(t, "dead-inst", 19876)

	instID := "dead-inst"
	if _, err := env.server.users.Update("alice", nil, &instID); err != nil {
		t.Fatal(err)
	}

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	cookie := extractSessionCookie(w)

	w = doCloudRequest(t, env, "GET", "/cloud/test", "", cookie)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(w.Body.String(), "unreachable") {
		t.Errorf("body should mention 'unreachable', got: %s", w.Body.String())
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
}

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
