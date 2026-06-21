package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
)

type rolloutTrackingPlugin struct {
	mu          sync.Mutex
	rolloutErr  error
	rollbackErr error
	rolledOut   []plugins.ImageRef
	rolledBack  []plugins.ImageRef
}

func (p *rolloutTrackingPlugin) Provision(_ context.Context, _ plugins.ProvisionRequest) error {
	return nil
}
func (p *rolloutTrackingPlugin) HealthCheck(_ context.Context, _ string) (plugins.HealthStatus, error) {
	return plugins.HealthStatus{Healthy: true, Ready: true, CheckedAt: time.Now().UTC()}, nil
}
func (p *rolloutTrackingPlugin) Configure(_ context.Context, _ string, _ plugins.InstanceConfig) error {
	return nil
}
func (p *rolloutTrackingPlugin) Destroy(_ context.Context, _ string) error { return nil }
func (p *rolloutTrackingPlugin) Restart(_ context.Context, _ string) error { return nil }
func (p *rolloutTrackingPlugin) Stats(_ context.Context, _ string) (plugins.ContainerStats, error) {
	return plugins.ContainerStats{}, nil
}
func (p *rolloutTrackingPlugin) Logs(_ context.Context, _ string, _ plugins.LogOpts) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (p *rolloutTrackingPlugin) Rollout(_ context.Context, _ string, target plugins.ImageRef) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rolledOut = append(p.rolledOut, target)
	return p.rolloutErr
}
func (p *rolloutTrackingPlugin) Rollback(_ context.Context, _ string, prev plugins.ImageRef) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rolledBack = append(p.rolledBack, prev)
	return p.rollbackErr
}

func newUpgradeTestEnv(t *testing.T, plug plugins.DeploymentPlugin) *testEnv {
	t.Helper()
	dir := t.TempDir()
	reg, err := registry.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Close() })

	prov := &fakeProvisioner{provisionCh: make(chan struct{}, 1)}
	srv := NewServer(Deps{
		Registry:     reg,
		Provisioner:  prov,
		Plugin:       plug,
		AdminSecret:  testSecret,
		InstancePwd:  "test-instance-pwd",
		MaxInstances: 10,
		PollInterval: time.Hour,
		SigningKey:   testSigningKey,
	})

	return &testEnv{
		server:   srv,
		registry: reg,
		prov:     prov,
	}
}

func TestUpgrade_Success(t *testing.T) {
	plug := &rolloutTrackingPlugin{}
	env := newUpgradeTestEnv(t, plug)
	env.seedInstance(t, "fox-1", 9100)

	w := env.doRequest("POST", "/api/instances/fox-1/upgrade",
		`{"target_image":"ghcr.io/fox-in-the-box-ai/cloud@sha256:newdigest"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp upgradeResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.ID != "fox-1" {
		t.Errorf("id = %q, want %q", resp.ID, "fox-1")
	}
	if resp.Status != "upgraded" {
		t.Errorf("status = %q, want %q", resp.Status, "upgraded")
	}
	if resp.PreviousDigest != "sha256:abc123" {
		t.Errorf("previous_digest = %q, want %q", resp.PreviousDigest, "sha256:abc123")
	}
	if resp.CurrentDigest != "sha256:newdigest" {
		t.Errorf("current_digest = %q, want %q", resp.CurrentDigest, "sha256:newdigest")
	}

	plug.mu.Lock()
	defer plug.mu.Unlock()
	if len(plug.rolledOut) != 1 {
		t.Fatalf("rollout calls = %d, want 1", len(plug.rolledOut))
	}
	if plug.rolledOut[0].Digest != "sha256:newdigest" {
		t.Errorf("rollout target digest = %q, want %q", plug.rolledOut[0].Digest, "sha256:newdigest")
	}

	inst, err := env.registry.Get("fox-1")
	if err != nil {
		t.Fatal(err)
	}
	if inst.ImageDigest != "sha256:newdigest" {
		t.Errorf("registry digest = %q, want %q", inst.ImageDigest, "sha256:newdigest")
	}
}

func TestUpgrade_AlreadyCurrent(t *testing.T) {
	plug := &rolloutTrackingPlugin{}
	env := newUpgradeTestEnv(t, plug)
	env.seedInstance(t, "fox-1", 9100)

	w := env.doRequest("POST", "/api/instances/fox-1/upgrade",
		`{"target_image":"ghcr.io/fox-in-the-box-ai/cloud@sha256:abc123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp upgradeResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != "already_current" {
		t.Errorf("status = %q, want %q", resp.Status, "already_current")
	}

	plug.mu.Lock()
	defer plug.mu.Unlock()
	if len(plug.rolledOut) != 0 {
		t.Errorf("rollout should not have been called, got %d calls", len(plug.rolledOut))
	}
}

func TestUpgrade_FailureTriggersRollback(t *testing.T) {
	plug := &rolloutTrackingPlugin{rolloutErr: fmt.Errorf("pull failed")}
	env := newUpgradeTestEnv(t, plug)
	env.seedInstance(t, "fox-1", 9100)

	w := env.doRequest("POST", "/api/instances/fox-1/upgrade",
		`{"target_image":"ghcr.io/fox-in-the-box-ai/cloud@sha256:newdigest"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusInternalServerError, w.Body.String())
	}

	var resp apiError
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != "upgrade_failed" {
		t.Errorf("error = %q, want %q", resp.Error, "upgrade_failed")
	}

	plug.mu.Lock()
	defer plug.mu.Unlock()
	if len(plug.rolledBack) != 1 {
		t.Fatalf("rollback calls = %d, want 1", len(plug.rolledBack))
	}
	if plug.rolledBack[0].Digest != "sha256:abc123" {
		t.Errorf("rollback digest = %q, want %q (previous digest)", plug.rolledBack[0].Digest, "sha256:abc123")
	}

	inst, err := env.registry.Get("fox-1")
	if err != nil {
		t.Fatal(err)
	}
	if inst.ImageDigest != "sha256:abc123" {
		t.Errorf("registry digest should be unchanged = %q, want %q", inst.ImageDigest, "sha256:abc123")
	}
}

func TestUpgrade_NotFound(t *testing.T) {
	plug := &rolloutTrackingPlugin{}
	env := newUpgradeTestEnv(t, plug)

	w := env.doRequest("POST", "/api/instances/nonexistent/upgrade",
		`{"target_image":"ghcr.io/fox-in-the-box-ai/cloud@sha256:abc"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUpgrade_InvalidInstanceID(t *testing.T) {
	plug := &rolloutTrackingPlugin{}
	env := newUpgradeTestEnv(t, plug)

	w := env.doRequest("POST", "/api/instances/$invalid!/upgrade",
		`{"target_image":"ghcr.io/fox-in-the-box-ai/cloud@sha256:abc"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpgrade_MissingTargetImage(t *testing.T) {
	plug := &rolloutTrackingPlugin{}
	env := newUpgradeTestEnv(t, plug)
	env.seedInstance(t, "fox-1", 9100)

	w := env.doRequest("POST", "/api/instances/fox-1/upgrade", `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestUpgrade_InvalidTargetImage(t *testing.T) {
	plug := &rolloutTrackingPlugin{}
	env := newUpgradeTestEnv(t, plug)
	env.seedInstance(t, "fox-1", 9100)

	w := env.doRequest("POST", "/api/instances/fox-1/upgrade",
		`{"target_image":"https://ghcr.io/foo/bar"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestUpgrade_InvalidJSON(t *testing.T) {
	plug := &rolloutTrackingPlugin{}
	env := newUpgradeTestEnv(t, plug)
	env.seedInstance(t, "fox-1", 9100)

	w := env.doRequest("POST", "/api/instances/fox-1/upgrade", `not json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpgrade_Idempotency_InFlight(t *testing.T) {
	plug := &rolloutTrackingPlugin{}
	env := newUpgradeTestEnv(t, plug)
	env.seedInstance(t, "fox-1", 9100)

	env.server.inFlightMu.Lock()
	env.server.inFlight["fox-1"] = true
	env.server.inFlightMu.Unlock()

	w := env.doRequest("POST", "/api/instances/fox-1/upgrade",
		`{"target_image":"ghcr.io/fox-in-the-box-ai/cloud@sha256:newdigest"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusConflict)
	}
}

func TestUpgrade_AuthRequired(t *testing.T) {
	plug := &rolloutTrackingPlugin{}
	env := newUpgradeTestEnv(t, plug)
	env.seedInstance(t, "fox-1", 9100)

	w := env.doRequestNoAuth("POST", "/api/instances/fox-1/upgrade")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestUpgrade_TagImage(t *testing.T) {
	plug := &rolloutTrackingPlugin{}
	env := newUpgradeTestEnv(t, plug)
	env.seedInstance(t, "fox-1", 9100)

	w := env.doRequest("POST", "/api/instances/fox-1/upgrade",
		`{"target_image":"ghcr.io/fox-in-the-box-ai/cloud:v0.7.58"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp upgradeResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != "upgraded" {
		t.Errorf("status = %q, want %q", resp.Status, "upgraded")
	}

	plug.mu.Lock()
	defer plug.mu.Unlock()
	if len(plug.rolledOut) != 1 {
		t.Fatalf("rollout calls = %d, want 1", len(plug.rolledOut))
	}
	if plug.rolledOut[0].Repository != "ghcr.io/fox-in-the-box-ai/cloud:v0.7.58" {
		t.Errorf("rollout repo = %q, want tag-inclusive ref", plug.rolledOut[0].Repository)
	}
}

func TestParseImageRef(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
		repo    string
		digest  string
	}{
		{
			input:  "ghcr.io/fox-in-the-box-ai/cloud@sha256:abc123",
			repo:   "ghcr.io/fox-in-the-box-ai/cloud",
			digest: "sha256:abc123",
		},
		{
			input:  "ghcr.io/fox-in-the-box-ai/cloud:v0.7.58",
			repo:   "ghcr.io/fox-in-the-box-ai/cloud:v0.7.58",
			digest: "",
		},
		{
			input:  "ghcr.io/fox-in-the-box-ai/cloud",
			repo:   "ghcr.io/fox-in-the-box-ai/cloud",
			digest: "",
		},
		{
			input:   "notaregistry",
			wantErr: true,
		},
		{
			input:   "https://ghcr.io/foo/bar",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ref, err := parseImageRef(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ref.Repository != tt.repo {
				t.Errorf("repo = %q, want %q", ref.Repository, tt.repo)
			}
			if ref.Digest != tt.digest {
				t.Errorf("digest = %q, want %q", ref.Digest, tt.digest)
			}
		})
	}
}
