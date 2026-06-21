package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/cloud"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/provisioner"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
)

// --- provision endpoint tests ---

func TestProvision_Success(t *testing.T) {
	env, users := newTestEnvWithUsers(t)

	w := env.doRequest("POST", "/api/instances/provision",
		`{"username":"alice","password":"password123","slug":"alice"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp provisionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.InstanceID != "alice" {
		t.Errorf("instance_id = %q, want %q", resp.InstanceID, "alice")
	}
	if resp.Username != "alice" {
		t.Errorf("username = %q, want %q", resp.Username, "alice")
	}
	if resp.Status != "provisioning" {
		t.Errorf("status = %q, want %q", resp.Status, "provisioning")
	}

	u, err := users.Get("alice")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if u.Username != "alice" {
		t.Errorf("username = %q, want %q", u.Username, "alice")
	}

	select {
	case <-env.prov.provisionCh:
	case <-time.After(2 * time.Second):
		t.Fatal("provisioner not called within timeout")
	}

	env.prov.mu.Lock()
	defer env.prov.mu.Unlock()
	if len(env.prov.provisions) != 1 {
		t.Fatalf("expected 1 provision call, got %d", len(env.prov.provisions))
	}
	if env.prov.provisions[0].InstanceID != "alice" {
		t.Errorf("provisioned ID = %q, want %q", env.prov.provisions[0].InstanceID, "alice")
	}
	if env.prov.provisions[0].Cloud.Slug != "alice" {
		t.Errorf("cloud.Slug = %q, want %q", env.prov.provisions[0].Cloud.Slug, "alice")
	}
}

func TestProvision_MissingUsername_DefaultsToSlug(t *testing.T) {
	env, users := newTestEnvWithUsers(t)

	w := env.doRequest("POST", "/api/instances/provision",
		`{"password":"password123","slug":"test"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp provisionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Username != "test" {
		t.Errorf("username = %q, want %q (should default to slug)", resp.Username, "test")
	}
	if resp.InstanceID != "test" {
		t.Errorf("instance_id = %q, want %q", resp.InstanceID, "test")
	}

	u, err := users.Get("test")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if u.Username != "test" {
		t.Errorf("username = %q, want %q", u.Username, "test")
	}
}

func TestProvision_MissingPassword(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequest("POST", "/api/instances/provision",
		`{"username":"test","slug":"test"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestProvision_MissingSlug(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequest("POST", "/api/instances/provision",
		`{"username":"alice","password":"password123"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestProvision_ShortPassword(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequest("POST", "/api/instances/provision",
		`{"username":"test","password":"short","slug":"test"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestProvision_LongPassword(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	long := strings.Repeat("a", 73)
	w := env.doRequest("POST", "/api/instances/provision",
		`{"username":"test","password":"`+long+`","slug":"test"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestProvision_UsernameMismatchRejected(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequest("POST", "/api/instances/provision",
		`{"username":"different","password":"password123","slug":"test"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	var resp apiError
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != "bad_request" {
		t.Errorf("error = %q, want %q", resp.Error, "bad_request")
	}
	if !strings.Contains(resp.Message, "username must equal slug") {
		t.Errorf("message = %q, want it to mention slug=username invariant", resp.Message)
	}
}

func TestProvision_InvalidSlug_Uppercase(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequest("POST", "/api/instances/provision",
		`{"password":"password123","slug":"Alice"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestProvision_InvalidSlug_LeadingHyphen(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequest("POST", "/api/instances/provision",
		`{"password":"password123","slug":"-test"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestProvision_InvalidSlug_TrailingHyphen(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequest("POST", "/api/instances/provision",
		`{"password":"password123","slug":"test-"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestProvision_InvalidSlug_ConsecutiveHyphens(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequest("POST", "/api/instances/provision",
		`{"password":"password123","slug":"te--st"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestProvision_ReservedSlug(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	for _, slug := range []string{"admin", "api", "login", "cloud", "healthz"} {
		w := env.doRequest("POST", "/api/instances/provision",
			`{"password":"password123","slug":"`+slug+`"}`)
		if w.Code != http.StatusConflict {
			t.Errorf("slug %q: status = %d, want %d", slug, w.Code, http.StatusConflict)
		}
		var resp apiError
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if resp.Error != "slug_reserved" {
			t.Errorf("slug %q: error = %q, want %q", slug, resp.Error, "slug_reserved")
		}
	}
}

func TestProvision_SlugTaken(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)
	env.seedInstance(t, "taken", 9100)

	w := env.doRequest("POST", "/api/instances/provision",
		`{"username":"taken","password":"password123","slug":"taken"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusConflict, w.Body.String())
	}
	var resp apiError
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != "slug_taken" {
		t.Errorf("error = %q, want %q", resp.Error, "slug_taken")
	}
}

func TestProvision_UserExists(t *testing.T) {
	env, users := newTestEnvWithUsers(t)

	if _, err := users.Create("alice", "password123"); err != nil {
		t.Fatal(err)
	}

	w := env.doRequest("POST", "/api/instances/provision",
		`{"username":"alice","password":"password123","slug":"alice"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusConflict, w.Body.String())
	}
	var resp apiError
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != "user_exists" {
		t.Errorf("error = %q, want %q", resp.Error, "user_exists")
	}
}

func TestProvision_CapReached(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)
	env.seedInstance(t, "fox-1", 9100)
	env.seedInstance(t, "fox-2", 9101)

	w := env.doRequest("POST", "/api/instances/provision",
		`{"username":"fox3","password":"password123","slug":"fox3"}`)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
}

func TestProvision_InvalidJSON(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequest("POST", "/api/instances/provision", `not json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestProvision_Unauthorized(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequestNoAuth("POST", "/api/instances/provision")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// --- slug check endpoint tests ---

func TestCheckSlug_Available(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequest("GET", "/api/slugs/check/myfox", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp slugCheckResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Available {
		t.Errorf("expected available=true, got false; reason=%q", resp.Reason)
	}
	if resp.Slug != "myfox" {
		t.Errorf("slug = %q, want %q", resp.Slug, "myfox")
	}
}

func TestCheckSlug_Reserved(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequest("GET", "/api/slugs/check/admin", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp slugCheckResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Available {
		t.Error("expected available=false for reserved slug")
	}
	if resp.Reason != "reserved" {
		t.Errorf("reason = %q, want %q", resp.Reason, "reserved")
	}
}

func TestCheckSlug_AlreadyInUse(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)
	env.seedInstance(t, "taken", 9100)

	w := env.doRequest("GET", "/api/slugs/check/taken", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp slugCheckResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Available {
		t.Error("expected available=false for taken slug")
	}
	if resp.Reason != "already in use" {
		t.Errorf("reason = %q, want %q", resp.Reason, "already in use")
	}
}

func TestCheckSlug_InvalidDNS(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	for _, slug := range []string{"-bad", "bad-", "ba--d", "BAD"} {
		path := "/api/slugs/check/" + slug
		w := env.doRequest("GET", path, "")
		if w.Code != http.StatusOK {
			t.Errorf("slug %q: status = %d, want %d", slug, w.Code, http.StatusOK)
			continue
		}
		var resp slugCheckResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if resp.Available {
			t.Errorf("slug %q: expected available=false", slug)
		}
	}
}

func TestCheckSlug_SingleChar(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequest("GET", "/api/slugs/check/a", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var resp slugCheckResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Available {
		t.Errorf("single char 'a' should be available, reason=%q", resp.Reason)
	}
}

func TestCheckSlug_Unauthorized(t *testing.T) {
	env, _ := newTestEnvWithUsers(t)

	w := env.doRequestNoAuth("GET", "/api/slugs/check/test")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// --- slug validation unit tests ---

func TestIsValidSlug(t *testing.T) {
	cases := []struct {
		slug string
		want bool
	}{
		{"a", true},
		{"abc", true},
		{"abc123", true},
		{"my-fox", true},
		{"a1b2c3", true},
		{"x", true},
		{"1", true},

		{"", false},
		{"-abc", false},
		{"abc-", false},
		{"ab--cd", false},
		{"ABC", false},
		{"aBc", false},
		{"a_b", false},
		{"a.b", false},
		{"a b", false},
	}

	for _, tc := range cases {
		got := isValidSlug(tc.slug)
		if got != tc.want {
			t.Errorf("isValidSlug(%q) = %v, want %v", tc.slug, got, tc.want)
		}
	}
}

func TestReservedSlugs_Complete(t *testing.T) {
	expected := []string{
		"admin", "api", "www", "mail", "login", "logout",
		"cloud", "fleet", "fox", "static", "assets",
		"healthz", "robots",
	}
	for _, s := range expected {
		if !reservedSlugs[s] {
			t.Errorf("expected %q to be reserved", s)
		}
	}
	if len(reservedSlugs) != len(expected) {
		t.Errorf("reservedSlugs has %d entries, expected %d", len(reservedSlugs), len(expected))
	}
}

func TestProvision_NoUserStoreReturns404(t *testing.T) {
	env := newTestEnv(t)

	w := env.doRequest("POST", "/api/instances/provision",
		`{"username":"test","password":"password123","slug":"test"}`)
	if w.Code == http.StatusCreated {
		t.Fatal("expected non-201 when UserStore is nil")
	}
}

func TestProvision_BindsUserAfterProvision(t *testing.T) {
	dir := t.TempDir()
	reg, err := registry.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Close() })

	users := cloud.NewUserStore(reg.DB())

	// A provisioner that writes the instance to the registry, like the real one.
	regProv := &registryAwareProvisioner{reg: reg, ch: make(chan struct{}, 1)}

	var logBuf bytes.Buffer
	srv := NewServer(Deps{
		Registry:     reg,
		Provisioner:  regProv,
		Plugin:       &fakePlugin{},
		AdminSecret:  testSecret,
		InstancePwd:  "test-instance-pwd",
		MaxInstances: 5,
		PollInterval: time.Hour,
		SigningKey:   testSigningKey,
		Logger:       slog.New(slog.NewTextHandler(&logBuf, nil)),
		UserStore:    users,
	})

	env := &testEnv{server: srv, registry: reg, logBuf: &logBuf}

	w := env.doRequest("POST", "/api/instances/provision",
		`{"username":"bob","password":"password123","slug":"bob"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusCreated, w.Body.String())
	}

	select {
	case <-regProv.ch:
	case <-time.After(2 * time.Second):
		t.Fatal("provisioner not called within timeout")
	}
	srv.Wait()

	u, err := users.Get("bob")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if u.InstanceID == nil || *u.InstanceID != "bob" {
		t.Errorf("instance_id = %v, want %q", u.InstanceID, "bob")
	}
}

type registryAwareProvisioner struct {
	reg *registry.Registry
	ch  chan struct{}
}

func (r *registryAwareProvisioner) Provision(_ context.Context, req provisioner.Request) (*provisioner.Instance, error) {
	err := r.reg.Create(registry.Instance{
		ID:        req.InstanceID,
		Port:      9200,
		DataDir:   "/data/" + req.InstanceID,
		Status:    "running",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, err
	}
	if r.ch != nil {
		r.ch <- struct{}{}
	}
	return &provisioner.Instance{
		ID:     req.InstanceID,
		Port:   9200,
		Status: "running",
	}, nil
}

func (r *registryAwareProvisioner) Destroy(_ context.Context, id string, _ bool) error {
	return nil
}

func TestProvision_UserCleanupOnSlugConflict(t *testing.T) {
	env, users := newTestEnvWithUsers(t)
	env.seedInstance(t, "taken", 9100)

	w := env.doRequest("POST", "/api/instances/provision",
		`{"username":"taken","password":"password123","slug":"taken"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusConflict)
	}

	_, err := users.Get("taken")
	if err != cloud.ErrUserNotFound {
		t.Errorf("expected user not found after slug conflict, got err=%v", err)
	}
}

func TestCreate_AutoBindsOwnerInCloudMode(t *testing.T) {
	dir := t.TempDir()
	reg, err := registry.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Close() })

	users := cloud.NewUserStore(reg.DB())
	if _, err := users.Create("dennis", "password123"); err != nil {
		t.Fatal(err)
	}

	regProv := &registryAwareProvisioner{reg: reg, ch: make(chan struct{}, 1)}

	var logBuf bytes.Buffer
	srv := NewServer(Deps{
		Registry:     reg,
		Provisioner:  regProv,
		Plugin:       &fakePlugin{},
		AdminSecret:  testSecret,
		InstancePwd:  "test-instance-pwd",
		MaxInstances: 5,
		PollInterval: time.Hour,
		SigningKey:   testSigningKey,
		Logger:       slog.New(slog.NewTextHandler(&logBuf, nil)),
		UserStore:    users,
		Cloud:        CloudConfig{Domain: "test.example.com"},
	})

	env := &testEnv{server: srv, registry: reg, logBuf: &logBuf}

	w := env.doRequest("POST", "/api/instances",
		`{"id":"dennis","owner":"dennis"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusCreated, w.Body.String())
	}

	select {
	case <-regProv.ch:
	case <-time.After(2 * time.Second):
		t.Fatal("provisioner not called within timeout")
	}
	srv.Wait()

	u, err := users.Get("dennis")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if u.InstanceID == nil || *u.InstanceID != "dennis" {
		t.Errorf("instance_id = %v, want %q", u.InstanceID, "dennis")
	}
}
