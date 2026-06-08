package rollout

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
)

var oldImage = plugins.ImageRef{Repository: "ghcr.io/fox-in-the-box-ai/fox", Digest: "sha256:old111"}
var newImage = plugins.ImageRef{Repository: "ghcr.io/fox-in-the-box-ai/fox", Digest: "sha256:new222"}

type fakePlugin struct {
	rolloutErr  map[string]error
	rollbackErr map[string]error
	healthErr   map[string]error
	unhealthy   map[string]bool

	rolledOut  []string
	rolledBack []string
}

func newFakePlugin() *fakePlugin {
	return &fakePlugin{
		rolloutErr:  make(map[string]error),
		rollbackErr: make(map[string]error),
		healthErr:   make(map[string]error),
		unhealthy:   make(map[string]bool),
	}
}

func (f *fakePlugin) Provision(_ context.Context, _ plugins.ProvisionRequest) error { return nil }
func (f *fakePlugin) Configure(_ context.Context, _ string, _ plugins.InstanceConfig) error {
	return nil
}
func (f *fakePlugin) Destroy(_ context.Context, _ string) error { return nil }
func (f *fakePlugin) Logs(_ context.Context, _ string, _ plugins.LogOpts) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (f *fakePlugin) Rollout(_ context.Context, id string, _ plugins.ImageRef) error {
	f.rolledOut = append(f.rolledOut, id)
	if err, ok := f.rolloutErr[id]; ok {
		return err
	}
	return nil
}

func (f *fakePlugin) Rollback(_ context.Context, id string, _ plugins.ImageRef) error {
	f.rolledBack = append(f.rolledBack, id)
	if err, ok := f.rollbackErr[id]; ok {
		return err
	}
	return nil
}

func (f *fakePlugin) HealthCheck(_ context.Context, id string) (plugins.HealthStatus, error) {
	if err, ok := f.healthErr[id]; ok {
		return plugins.HealthStatus{}, err
	}
	return plugins.HealthStatus{
		Healthy:   !f.unhealthy[id],
		CheckedAt: time.Now().UTC(),
	}, nil
}

func newTestRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	dir := t.TempDir()
	reg, err := registry.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Close() })
	return reg
}

func seedInstances(t *testing.T, reg *registry.Registry, ids ...string) {
	t.Helper()
	for i, id := range ids {
		if err := reg.Create(registry.Instance{
			ID:          id,
			ImageDigest: oldImage.Digest,
			Port:        9100 + i,
			DataDir:     filepath.Join(os.TempDir(), id),
			Status:      "running",
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func newOrchestrator(reg *registry.Registry, plug *fakePlugin) *Orchestrator {
	return New(Options{
		Registry:       reg,
		Plugin:         plug,
		HealthTimeout:  2 * time.Second,
		HealthInterval: func(_ time.Duration) time.Duration { return 50 * time.Millisecond },
	})
}

func TestRolloutEmpty(t *testing.T) {
	reg := newTestRegistry(t)
	plug := newFakePlugin()
	orch := newOrchestrator(reg, plug)

	report, err := orch.Execute(context.Background(), newImage)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Instances) != 0 {
		t.Errorf("expected 0 instances, got %d", len(report.Instances))
	}
	if !report.OK() {
		t.Error("empty rollout should report OK")
	}
}

func TestRolloutSingleSuccess(t *testing.T) {
	reg := newTestRegistry(t)
	plug := newFakePlugin()
	seedInstances(t, reg, "fox-alpha")
	orch := newOrchestrator(reg, plug)

	report, err := orch.Execute(context.Background(), newImage)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK() {
		t.Errorf("expected OK, got instances: %+v", report.Instances)
	}
	if len(report.Instances) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Instances))
	}
	if report.Instances[0].Status != StatusRolledOut {
		t.Errorf("expected rolled_out, got %s", report.Instances[0].Status)
	}
	if report.Instances[0].Previous.Digest != oldImage.Digest {
		t.Errorf("expected previous digest %s, got %s", oldImage.Digest, report.Instances[0].Previous.Digest)
	}

	inst, err := reg.Get("fox-alpha")
	if err != nil {
		t.Fatal(err)
	}
	if inst.ImageDigest != newImage.Digest {
		t.Errorf("registry should be updated to %s, got %s", newImage.Digest, inst.ImageDigest)
	}
}

func TestRolloutMultipleSuccess(t *testing.T) {
	reg := newTestRegistry(t)
	plug := newFakePlugin()
	seedInstances(t, reg, "fox-alpha", "fox-beta", "fox-gamma")
	orch := newOrchestrator(reg, plug)

	report, err := orch.Execute(context.Background(), newImage)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK() {
		t.Errorf("expected OK, got instances: %+v", report.Instances)
	}
	if len(report.Instances) != 3 {
		t.Fatalf("expected 3 results, got %d", len(report.Instances))
	}
	for _, r := range report.Instances {
		if r.Status != StatusRolledOut {
			t.Errorf("instance %s: expected rolled_out, got %s", r.ID, r.Status)
		}
	}

	if len(plug.rolledOut) != 3 {
		t.Errorf("expected 3 rollout calls, got %d", len(plug.rolledOut))
	}
}

func TestRolloutFailureAbortsRemaining(t *testing.T) {
	reg := newTestRegistry(t)
	plug := newFakePlugin()
	seedInstances(t, reg, "fox-alpha", "fox-beta", "fox-gamma")

	plug.rolloutErr["fox-beta"] = fmt.Errorf("image pull failed")
	orch := newOrchestrator(reg, plug)

	report, err := orch.Execute(context.Background(), newImage)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK() {
		t.Error("expected failure")
	}
	if len(report.Instances) != 3 {
		t.Fatalf("expected 3 results, got %d", len(report.Instances))
	}

	if report.Instances[0].Status != StatusRolledOut {
		t.Errorf("alpha: expected rolled_out, got %s", report.Instances[0].Status)
	}
	if report.Instances[1].Status != StatusRolledBack {
		t.Errorf("beta: expected rolled_back, got %s", report.Instances[1].Status)
	}
	if report.Instances[2].Status != StatusSkipped {
		t.Errorf("gamma: expected skipped, got %s", report.Instances[2].Status)
	}

	alpha, err := reg.Get("fox-alpha")
	if err != nil {
		t.Fatal(err)
	}
	if alpha.ImageDigest != newImage.Digest {
		t.Errorf("alpha registry should be updated to new, got %s", alpha.ImageDigest)
	}
	beta, err := reg.Get("fox-beta")
	if err != nil {
		t.Fatal(err)
	}
	if beta.ImageDigest != oldImage.Digest {
		t.Errorf("beta registry should stay at old, got %s", beta.ImageDigest)
	}
}

func TestRolloutHealthFailureTriggersRollback(t *testing.T) {
	reg := newTestRegistry(t)
	plug := newFakePlugin()
	seedInstances(t, reg, "fox-alpha")

	plug.unhealthy["fox-alpha"] = true
	orch := newOrchestrator(reg, plug)

	report, err := orch.Execute(context.Background(), newImage)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK() {
		t.Error("expected failure due to health check")
	}
	if report.Instances[0].Status != StatusRolledBack {
		t.Errorf("expected rolled_back, got %s", report.Instances[0].Status)
	}
	if len(plug.rolledBack) != 1 {
		t.Errorf("expected 1 rollback call, got %d", len(plug.rolledBack))
	}

	inst, err := reg.Get("fox-alpha")
	if err != nil {
		t.Fatal(err)
	}
	if inst.ImageDigest != oldImage.Digest {
		t.Errorf("registry should remain at old digest after rollback, got %s", inst.ImageDigest)
	}
}

func TestRolloutContextCancelled(t *testing.T) {
	reg := newTestRegistry(t)
	plug := newFakePlugin()
	seedInstances(t, reg, "fox-alpha", "fox-beta")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	orch := newOrchestrator(reg, plug)

	report, err := orch.Execute(ctx, newImage)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Instances) != 2 {
		t.Fatalf("expected 2 instances in report, got %d", len(report.Instances))
	}
	for _, r := range report.Instances {
		if r.Status == StatusRolledOut {
			t.Errorf("no instance should be rolled out on cancelled context")
		}
		if r.Status != StatusSkipped {
			t.Errorf("instance %s: expected skipped, got %s", r.ID, r.Status)
		}
	}
}

func TestRolloutRollbackAlsoFails(t *testing.T) {
	reg := newTestRegistry(t)
	plug := newFakePlugin()
	seedInstances(t, reg, "fox-alpha")

	plug.unhealthy["fox-alpha"] = true
	plug.rollbackErr["fox-alpha"] = fmt.Errorf("disk full")
	orch := newOrchestrator(reg, plug)

	report, err := orch.Execute(context.Background(), newImage)
	if err != nil {
		t.Fatal(err)
	}
	if report.Instances[0].Status != StatusRolledBack {
		t.Errorf("expected rolled_back, got %s", report.Instances[0].Status)
	}
	if !strings.Contains(report.Instances[0].Error, "rollback failed") {
		t.Errorf("expected rollback failure in error, got: %s", report.Instances[0].Error)
	}
}

func TestReportFormat(t *testing.T) {
	report := &Report{
		Target: newImage,
		Instances: []InstanceResult{
			{ID: "fox-alpha", Status: StatusRolledOut, Previous: plugins.ImageRef{Digest: oldImage.Digest}},
			{ID: "fox-beta", Status: StatusRolledBack, Previous: plugins.ImageRef{Digest: oldImage.Digest}, Error: "health failed"},
		},
	}
	out := report.Format()
	if !strings.Contains(out, "fox-alpha") {
		t.Error("format should contain fox-alpha")
	}
	if !strings.Contains(out, "rolled_back") {
		t.Error("format should contain rolled_back")
	}
	if !strings.Contains(out, "health failed") {
		t.Error("format should contain error message")
	}
}

var _ plugins.DeploymentPlugin = (*fakePlugin)(nil)
