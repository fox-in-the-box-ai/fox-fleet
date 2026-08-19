package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/cloud"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
)

func newTestEnvWithCloud(t *testing.T) *testEnv {
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

	if _, err := users.Create("alice", "password123"); err != nil {
		t.Fatalf("create test user: %v", err)
	}

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
		SigningKey:   testSigningKey,
		Logger:       logger,
		UserStore:    users,
		SessionStore: sessions,
		Cloud: CloudConfig{
			CookieName:     "fox_cloud_session",
			Secure:         true,
			SessionTTL:     time.Hour,
			LoginRateLimit: 5,
		},
	})

	return &testEnv{
		server:   srv,
		registry: reg,
		prov:     prov,
		logBuf:   &logBuf,
	}
}

func doCloudRequest(t *testing.T, env *testEnv, method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	env.server.Handler().ServeHTTP(w, r)
	return w
}

func extractSessionCookie(w *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == "fox_cloud_session" {
			return c
		}
	}
	return nil
}

func TestCloudAuth_LoginSuccess(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["username"] != "alice" {
		t.Errorf("username = %q, want %q", resp["username"], "alice")
	}

	cookie := extractSessionCookie(w)
	if cookie == nil {
		t.Fatal("no session cookie set")
		return // unreachable; SA5011 toolchain regression
	}
	if !cookie.Secure {
		t.Error("cookie should be Secure in cloud mode")
	}
	if !cookie.HttpOnly {
		t.Error("cookie should be HttpOnly")
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Error("cookie should be SameSite=Strict")
	}
	if cookie.Path != "/" {
		t.Errorf("cookie Path = %q, want %q", cookie.Path, "/")
	}
	if cookie.Value == "" {
		t.Error("cookie value is empty")
	}
}

func TestCloudAuth_LoginInvalidPassword(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"wrongpassword"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if !strings.Contains(w.Body.String(), "invalid credentials") {
		t.Errorf("body = %s, want 'invalid credentials'", w.Body.String())
	}
}

func TestCloudAuth_LoginUnknownUser(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"nobody","password":"password123"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if !strings.Contains(w.Body.String(), "invalid credentials") {
		t.Errorf("body = %s, want 'invalid credentials'", w.Body.String())
	}
}

func TestCloudAuth_LoginMissingFields(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCloudAuth_LoginUsernameTooLong(t *testing.T) {
	env := newTestEnvWithCloud(t)

	longUser := strings.Repeat("a", 65)
	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"`+longUser+`","password":"password123"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCloudAuth_LoginPasswordTooLong(t *testing.T) {
	env := newTestEnvWithCloud(t)

	longPwd := strings.Repeat("x", 73)
	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"`+longPwd+`"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCloudAuth_LoginInvalidJSON(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{invalid`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCloudAuth_Logout(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login: status = %d", w.Code)
	}
	cookie := extractSessionCookie(w)
	if cookie == nil {
		t.Fatal("no session cookie after login")
	}

	w = doCloudRequest(t, env, "POST", "/cloud/logout", "", cookie)
	if w.Code != http.StatusNoContent {
		t.Fatalf("logout: status = %d, want %d; body = %s", w.Code, http.StatusNoContent, w.Body.String())
	}

	clearCookie := extractSessionCookie(w)
	if clearCookie == nil {
		t.Fatal("no cookie-clear header on logout")
		return // unreachable; SA5011 toolchain regression
	}
	if clearCookie.MaxAge >= 0 {
		t.Errorf("MaxAge = %d, want negative (cookie deletion)", clearCookie.MaxAge)
	}
}

func TestCloudAuth_LogoutNoCookie(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "POST", "/cloud/logout", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestCloudAuth_SessionMiddleware(t *testing.T) {
	env := newTestEnvWithCloud(t)

	handler := env.server.requireCloudSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := CloudUserFromContext(r.Context())
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"user": u.Username})
	}))

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	cookie := extractSessionCookie(w)

	r := httptest.NewRequest("GET", "/test", nil)
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["user"] != "alice" {
		t.Errorf("user = %q, want %q", resp["user"], "alice")
	}
}

func TestCloudAuth_SessionMiddlewareNoCookie(t *testing.T) {
	env := newTestEnvWithCloud(t)

	handler := env.server.requireCloudSession(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCloudAuth_SessionMiddlewareInvalidToken(t *testing.T) {
	env := newTestEnvWithCloud(t)

	handler := env.server.requireCloudSession(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest("GET", "/test", nil)
	r.AddCookie(&http.Cookie{Name: "fox_cloud_session", Value: "invalid-token"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCloudAuth_SessionAfterLogout(t *testing.T) {
	env := newTestEnvWithCloud(t)

	w := doCloudRequest(t, env, "POST", "/cloud/login", `{"username":"alice","password":"password123"}`)
	cookie := extractSessionCookie(w)

	doCloudRequest(t, env, "POST", "/cloud/logout", "", cookie)

	handler := env.server.requireCloudSession(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	r := httptest.NewRequest("GET", "/test", nil)
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, r)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d (session should be invalidated after logout)", rec.Code, http.StatusUnauthorized)
	}
}
