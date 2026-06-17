package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/cloud"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
)

func newTestEnvWithUsers(t *testing.T) (*testEnv, *cloud.UserStore) {
	t.Helper()

	dir := t.TempDir()
	reg, err := registry.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Close() })

	users := cloud.NewUserStore(reg.DB())

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
	})

	return &testEnv{
		server:   srv,
		registry: reg,
		prov:     prov,
		logBuf:   &logBuf,
	}, users
}

func TestUserHandlers_CreateUser(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequest("POST", "/api/users", `{"username":"alice","password":"password123"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusCreated, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var u cloud.User
	if err := json.NewDecoder(w.Body).Decode(&u); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if u.Username != "alice" {
		t.Errorf("username = %q, want %q", u.Username, "alice")
	}
}

func TestUserHandlers_CreateUserDuplicate(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	env.doRequest("POST", "/api/users", `{"username":"alice","password":"password123"}`)
	w := env.doRequest("POST", "/api/users", `{"username":"alice","password":"different123"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusConflict)
	}
}

func TestUserHandlers_CreateUserMissingUsername(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequest("POST", "/api/users", `{"password":"password123"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUserHandlers_CreateUserShortPassword(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequest("POST", "/api/users", `{"username":"alice","password":"short"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUserHandlers_CreateUserLongUsername(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	long := strings.Repeat("a", 65)
	w := env.doRequest("POST", "/api/users", `{"username":"`+long+`","password":"password123"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUserHandlers_ListUsers(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequest("GET", "/api/users", "")
	if w.Code != http.StatusOK {
		t.Fatalf("empty list: status = %d, want %d", w.Code, http.StatusOK)
	}
	if body := strings.TrimSpace(w.Body.String()); body != "[]" {
		t.Errorf("empty list: body = %q, want %q", body, "[]")
	}

	env.doRequest("POST", "/api/users", `{"username":"alice","password":"password123"}`)
	env.doRequest("POST", "/api/users", `{"username":"bob","password":"password456"}`)

	w = env.doRequest("GET", "/api/users", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var users []cloud.User
	if err := json.NewDecoder(w.Body).Decode(&users); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("len(users) = %d, want 2", len(users))
	}
}

func TestUserHandlers_GetUser(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	env.doRequest("POST", "/api/users", `{"username":"alice","password":"password123"}`)

	w := env.doRequest("GET", "/api/users/alice", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestUserHandlers_GetUserNotFound(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequest("GET", "/api/users/nobody", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUserHandlers_UpdateUser(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	env.doRequest("POST", "/api/users", `{"username":"alice","password":"password123"}`)

	w := env.doRequest("PUT", "/api/users/alice", `{"password":"newpassword123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestUserHandlers_UpdateUserNotFound(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequest("PUT", "/api/users/nobody", `{"password":"newpassword123"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUserHandlers_UpdateUserShortPassword(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	env.doRequest("POST", "/api/users", `{"username":"alice","password":"password123"}`)

	w := env.doRequest("PUT", "/api/users/alice", `{"password":"short"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUserHandlers_DeleteUser(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	env.doRequest("POST", "/api/users", `{"username":"alice","password":"password123"}`)

	w := env.doRequest("DELETE", "/api/users/alice", "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
	}

	w = env.doRequest("GET", "/api/users/alice", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("after delete: status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUserHandlers_DeleteUserNotFound(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequest("DELETE", "/api/users/nobody", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUserHandlers_Unauthorized(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequestNoAuth("GET", "/api/users")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestUserHandlers_CreateUserInvalidJSON(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequest("POST", "/api/users", `{invalid`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUserHandlers_UpdateUserInstanceID(t *testing.T) {
	env, users := newTestEnvWithUsers(t)

	env.doRequest("POST", "/api/users", `{"username":"alice","password":"password123"}`)

	if err := env.registry.Create(registry.Instance{
		ID: "test-inst", Port: 9999, DataDir: t.TempDir(), Status: "running",
	}); err != nil {
		t.Fatalf("create instance: %v", err)
	}

	if err := users.SetInstanceID("alice", "test-inst"); err != nil {
		t.Fatalf("set instance_id via store: %v", err)
	}

	w := env.doRequest("GET", "/api/users/alice", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var u cloud.User
	if err := json.NewDecoder(w.Body).Decode(&u); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if u.InstanceID == nil || *u.InstanceID != "test-inst" {
		t.Errorf("instance_id = %v, want %q", u.InstanceID, "test-inst")
	}
}

func TestUserHandlers_InvalidUsername(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequest("GET", "/api/users/bad%20user!", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}
