package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudIntegration_FullLifecycle(t *testing.T) {
	env := newTestEnvWithCloud(t)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello from instance")
	}))
	defer backend.Close()

	port := parseTestPort(t, backend.URL)
	env.seedInstance(t, "life-inst", port)

	// Admin creates user via API
	w := env.doRequest("POST", "/api/users", `{"username":"lifecycle","password":"password123"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create user: status = %d, body = %s", w.Code, w.Body.String())
	}

	// Admin assigns instance to user
	instID := "life-inst"
	if _, err := env.server.users.Update("lifecycle", nil, &instID); err != nil {
		t.Fatalf("assign instance: %v", err)
	}

	// User logs in
	w = doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"lifecycle","password":"password123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login: status = %d, body = %s", w.Code, w.Body.String())
	}
	cookie := extractSessionCookie(w)
	if cookie == nil {
		t.Fatal("no session cookie after login")
	}

	// Proxy works
	w = doCloudRequest(t, env, "GET", "/cloud/app", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("proxy: status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "hello from instance") {
		t.Errorf("proxy body = %q, want 'hello from instance'", w.Body.String())
	}

	// User logs out
	w = doCloudRequest(t, env, "POST", "/cloud/logout", "", cookie)
	if w.Code != http.StatusNoContent {
		t.Fatalf("logout: status = %d", w.Code)
	}

	// Proxy redirects after logout
	w = doCloudRequest(t, env, "GET", "/cloud/app", "", cookie)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("post-logout proxy: status = %d, want %d", w.Code, http.StatusSeeOther)
	}
}

func TestCloudIntegration_DeleteUserInvalidatesAccess(t *testing.T) {
	env := newTestEnvWithCloud(t)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	port := parseTestPort(t, backend.URL)
	env.seedInstance(t, "del-inst", port)

	instID := "del-inst"
	if _, err := env.server.users.Update("alice", nil, &instID); err != nil {
		t.Fatalf("assign instance: %v", err)
	}

	// Login
	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	cookie := extractSessionCookie(w)
	if cookie == nil {
		t.Fatal("no session cookie")
	}

	// Proxy works before deletion
	w = doCloudRequest(t, env, "GET", "/cloud/test", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("pre-delete proxy: status = %d", w.Code)
	}

	// Admin deletes user
	w = env.doRequest("DELETE", "/api/users/alice", "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete user: status = %d, body = %s", w.Code, w.Body.String())
	}

	// Proxy fails after user deletion (redirect to login)
	w = doCloudRequest(t, env, "GET", "/cloud/test", "", cookie)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("post-delete proxy: status = %d, want %d", w.Code, http.StatusSeeOther)
	}
}

func TestCloudIntegration_MultiUserIsolation(t *testing.T) {
	env := newTestEnvWithCloud(t)

	var gotPathA, gotPathB string
	backendA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPathA = r.URL.Path
		fmt.Fprintf(w, "instance-a")
	}))
	defer backendA.Close()

	backendB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPathB = r.URL.Path
		fmt.Fprintf(w, "instance-b")
	}))
	defer backendB.Close()

	portA := parseTestPort(t, backendA.URL)
	portB := parseTestPort(t, backendB.URL)
	env.seedInstance(t, "inst-a", portA)
	env.seedInstance(t, "inst-b", portB)

	// Create user B (alice already exists from newTestEnvWithCloud)
	if _, err := env.server.users.Create("bob", "password456"); err != nil {
		t.Fatalf("create bob: %v", err)
	}

	// Assign instances
	instA := "inst-a"
	instB := "inst-b"
	if _, err := env.server.users.Update("alice", nil, &instA); err != nil {
		t.Fatalf("assign alice: %v", err)
	}
	if _, err := env.server.users.Update("bob", nil, &instB); err != nil {
		t.Fatalf("assign bob: %v", err)
	}

	// Login both
	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	cookieA := extractSessionCookie(w)
	w = doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"bob","password":"password456"}`)
	cookieB := extractSessionCookie(w)

	if cookieA == nil || cookieB == nil {
		t.Fatal("missing session cookies")
	}

	// Alice proxies to instance-a
	w = doCloudRequest(t, env, "GET", "/cloud/data", "", cookieA)
	if w.Code != http.StatusOK {
		t.Fatalf("alice proxy: status = %d", w.Code)
	}
	if w.Body.String() != "instance-a" {
		t.Errorf("alice got %q, want instance-a", w.Body.String())
	}

	// Bob proxies to instance-b
	w = doCloudRequest(t, env, "GET", "/cloud/data", "", cookieB)
	if w.Code != http.StatusOK {
		t.Fatalf("bob proxy: status = %d", w.Code)
	}
	if w.Body.String() != "instance-b" {
		t.Errorf("bob got %q, want instance-b", w.Body.String())
	}

	// Both hit the same path
	if gotPathA != "/data" {
		t.Errorf("instance-a got path %q, want /data", gotPathA)
	}
	if gotPathB != "/data" {
		t.Errorf("instance-b got path %q, want /data", gotPathB)
	}
}

func TestCloudIntegration_AdminAPIAlongsideCloud(t *testing.T) {
	env := newTestEnvWithCloud(t)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	port := parseTestPort(t, backend.URL)
	env.seedInstance(t, "coexist-inst", port)

	instID := "coexist-inst"
	if _, err := env.server.users.Update("alice", nil, &instID); err != nil {
		t.Fatalf("assign instance: %v", err)
	}

	// Cloud login works
	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("cloud login: status = %d", w.Code)
	}
	cookie := extractSessionCookie(w)

	// Cloud proxy works
	w = doCloudRequest(t, env, "GET", "/cloud/test", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("cloud proxy: status = %d", w.Code)
	}

	// Admin API still works with bearer token
	w = env.doRequest("GET", "/api/instances", "")
	if w.Code != http.StatusOK {
		t.Fatalf("admin list instances: status = %d, body = %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/users", "")
	if w.Code != http.StatusOK {
		t.Fatalf("admin list users: status = %d, body = %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/users/alice", "")
	if w.Code != http.StatusOK {
		t.Fatalf("admin get user: status = %d, body = %s", w.Code, w.Body.String())
	}

	// Admin API rejects unauthenticated
	w = env.doRequestNoAuth("GET", "/api/instances")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("admin unauth: status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestCloudIntegration_AssignInstanceFlow(t *testing.T) {
	env := newTestEnvWithCloud(t)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "assigned")
	}))
	defer backend.Close()

	port := parseTestPort(t, backend.URL)
	env.seedInstance(t, "assign-inst", port)

	// Login
	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	cookie := extractSessionCookie(w)
	if cookie == nil {
		t.Fatal("no session cookie")
	}

	// Proxy returns 503 — no instance assigned
	w = doCloudRequest(t, env, "GET", "/cloud/test", "", cookie)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("pre-assign proxy: status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	// Admin assigns instance
	instID := "assign-inst"
	if _, err := env.server.users.Update("alice", nil, &instID); err != nil {
		t.Fatalf("assign instance: %v", err)
	}

	// Proxy works after assignment
	w = doCloudRequest(t, env, "GET", "/cloud/test", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("post-assign proxy: status = %d, body = %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "assigned" {
		t.Errorf("body = %q, want 'assigned'", w.Body.String())
	}
}

func TestCloudIntegration_PasswordChangeFlow(t *testing.T) {
	env := newTestEnvWithCloud(t)

	// Login with original password
	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("initial login: status = %d", w.Code)
	}

	// Admin changes password
	w = env.doRequest("PUT", "/api/users/alice", `{"password":"newpassword456"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("update password: status = %d, body = %s", w.Code, w.Body.String())
	}

	// Old password fails
	w = doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("old password login: status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	// New password succeeds
	w = doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"newpassword456"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("new password login: status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestCloudIntegration_LogoutInvalidatesSession(t *testing.T) {
	env := newTestEnvWithCloud(t)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	port := parseTestPort(t, backend.URL)
	env.seedInstance(t, "logout-inst", port)

	instID := "logout-inst"
	if _, err := env.server.users.Update("alice", nil, &instID); err != nil {
		t.Fatalf("assign instance: %v", err)
	}

	// Login
	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	cookie := extractSessionCookie(w)

	// Proxy works
	w = doCloudRequest(t, env, "GET", "/cloud/test", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("pre-logout proxy: status = %d", w.Code)
	}

	// Logout
	w = doCloudRequest(t, env, "POST", "/cloud/logout", "", cookie)
	if w.Code != http.StatusNoContent {
		t.Fatalf("logout: status = %d", w.Code)
	}

	// Same cookie now fails
	w = doCloudRequest(t, env, "GET", "/cloud/test", "", cookie)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("post-logout proxy: status = %d, want %d", w.Code, http.StatusSeeOther)
	}

	// Login page accessible
	w = doCloudRequest(t, env, "GET", "/cloud/login", "")
	if w.Code != http.StatusOK {
		t.Fatalf("login page: status = %d", w.Code)
	}
}

func TestCloudIntegration_MultipleSessionsSameUser(t *testing.T) {
	env := newTestEnvWithCloud(t)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	port := parseTestPort(t, backend.URL)
	env.seedInstance(t, "multi-sess-inst", port)

	instID := "multi-sess-inst"
	if _, err := env.server.users.Update("alice", nil, &instID); err != nil {
		t.Fatalf("assign instance: %v", err)
	}

	// Login twice — two independent sessions
	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	cookie1 := extractSessionCookie(w)

	w = doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	cookie2 := extractSessionCookie(w)

	if cookie1 == nil || cookie2 == nil {
		t.Fatal("missing session cookies")
	}
	if cookie1.Value == cookie2.Value {
		t.Fatal("two logins produced the same session token")
	}

	// Both sessions work
	w = doCloudRequest(t, env, "GET", "/cloud/test", "", cookie1)
	if w.Code != http.StatusOK {
		t.Fatalf("session1 proxy: status = %d", w.Code)
	}

	w = doCloudRequest(t, env, "GET", "/cloud/test", "", cookie2)
	if w.Code != http.StatusOK {
		t.Fatalf("session2 proxy: status = %d", w.Code)
	}

	// Logout session 1
	w = doCloudRequest(t, env, "POST", "/cloud/logout", "", cookie1)
	if w.Code != http.StatusNoContent {
		t.Fatalf("logout session1: status = %d", w.Code)
	}

	// Session 1 no longer works
	w = doCloudRequest(t, env, "GET", "/cloud/test", "", cookie1)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("post-logout session1: status = %d, want %d", w.Code, http.StatusSeeOther)
	}

	// Session 2 still works
	w = doCloudRequest(t, env, "GET", "/cloud/test", "", cookie2)
	if w.Code != http.StatusOK {
		t.Fatalf("session2 after logout1: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestCloudIntegration_DisabledCloudPreservesOldRouting(t *testing.T) {
	env := newTestEnv(t)

	// Root must NOT redirect to /cloud/login (cloud-mode behavior)
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	env.server.Handler().ServeHTTP(w, r)

	if w.Code == http.StatusSeeOther && w.Header().Get("Location") == "/cloud/login" {
		t.Fatal("root redirects to /cloud/login even with cloud disabled")
	}

	// Cloud login route must not be registered
	r = httptest.NewRequest("POST", "/cloud/login", strings.NewReader(`{"username":"alice","password":"password123"}`))
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	env.server.Handler().ServeHTTP(w, r)

	if w.Code == http.StatusOK || w.Code == http.StatusUnauthorized {
		t.Fatalf("POST /cloud/login returned %d — cloud routes should not exist when disabled", w.Code)
	}

	// Cloud proxy route must not be registered
	r = httptest.NewRequest("GET", "/cloud/anything", nil)
	w = httptest.NewRecorder()
	env.server.Handler().ServeHTTP(w, r)

	if w.Code == http.StatusSeeOther && w.Header().Get("Location") == "/cloud/login" {
		t.Fatal("/cloud/* redirects to login even with cloud disabled")
	}

	// Admin API still works
	w = env.doRequest("GET", "/api/instances", "")
	if w.Code != http.StatusOK {
		t.Fatalf("admin API with cloud disabled: status = %d", w.Code)
	}
}
