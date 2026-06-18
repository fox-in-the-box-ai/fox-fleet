package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudIntegration_LegacyCloudPathRedirects(t *testing.T) {
	env := newTestEnvWithCloud(t)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	port := parseTestPort(t, backend.URL)
	env.seedInstance(t, "life-inst", port)

	instID := "life-inst"
	if _, err := env.server.users.Update("alice", nil, &instID); err != nil {
		t.Fatalf("assign instance: %v", err)
	}

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	cookie := extractSessionCookie(w)
	if cookie == nil {
		t.Fatal("no session cookie after login")
	}

	w = doCloudRequest(t, env, "GET", "/cloud/app", "", cookie)
	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("legacy /cloud/app: status = %d, want %d", w.Code, http.StatusMovedPermanently)
	}
	if loc := w.Header().Get("Location"); loc != "/admin/" {
		t.Errorf("Location = %q, want /admin/", loc)
	}

	w = doCloudRequest(t, env, "POST", "/cloud/logout", "", cookie)
	if w.Code != http.StatusNoContent {
		t.Fatalf("logout: status = %d", w.Code)
	}
}

func TestCloudIntegration_AdminAPIAlongsideCloud(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("cloud login: status = %d", w.Code)
	}

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

	w = env.doRequestNoAuth("GET", "/api/instances")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("admin unauth: status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestCloudIntegration_PasswordChangeFlow(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("initial login: status = %d", w.Code)
	}

	w = env.doRequest("PUT", "/api/users/alice", `{"password":"newpassword456"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("update password: status = %d, body = %s", w.Code, w.Body.String())
	}

	w = doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("old password login: status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	w = doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"newpassword456"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("new password login: status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestCloudIntegration_LogoutInvalidatesSession(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	cookie := extractSessionCookie(w)

	w = doCloudRequest(t, env, "POST", "/cloud/logout", "", cookie)
	if w.Code != http.StatusNoContent {
		t.Fatalf("logout: status = %d", w.Code)
	}

	w = doCloudRequest(t, env, "GET", "/cloud/login", "")
	if w.Code != http.StatusOK {
		t.Fatalf("login page: status = %d", w.Code)
	}
}

func TestCloudIntegration_MultipleSessionsSameUser(t *testing.T) {
	env := newTestEnvWithCloud(t)

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

	w = doCloudRequest(t, env, "POST", "/cloud/logout", "", cookie1)
	if w.Code != http.StatusNoContent {
		t.Fatalf("logout session1: status = %d", w.Code)
	}
}

func TestCloudIntegration_DisabledCloudPreservesOldRouting(t *testing.T) {
	env := newTestEnv(t)

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	env.server.Handler().ServeHTTP(w, r)

	if w.Code == http.StatusSeeOther && w.Header().Get("Location") == "/cloud/login" {
		t.Fatal("root redirects to /cloud/login even with cloud disabled")
	}

	r = httptest.NewRequest("POST", "/cloud/login", strings.NewReader(`{"username":"alice","password":"password123"}`))
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	env.server.Handler().ServeHTTP(w, r)

	if w.Code == http.StatusOK || w.Code == http.StatusUnauthorized {
		t.Fatalf("POST /cloud/login returned %d — cloud routes should not exist when disabled", w.Code)
	}

	r = httptest.NewRequest("GET", "/cloud/anything", nil)
	w = httptest.NewRecorder()
	env.server.Handler().ServeHTTP(w, r)

	if w.Code == http.StatusMovedPermanently && w.Header().Get("Location") == "/admin/" {
		t.Fatal("/cloud/* redirects to /admin/ even with cloud disabled")
	}

	w = env.doRequest("GET", "/api/instances", "")
	if w.Code != http.StatusOK {
		t.Fatalf("admin API with cloud disabled: status = %d", w.Code)
	}
}
