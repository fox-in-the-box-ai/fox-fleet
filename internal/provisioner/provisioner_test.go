package provisioner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/config"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
)

type fakePlugin struct {
	mu           sync.Mutex
	provisionErr func(req plugins.ProvisionRequest) error
	destroyErr   error
	provisioned  []plugins.ProvisionRequest
	destroyed    []string
}

func (f *fakePlugin) Provision(_ context.Context, req plugins.ProvisionRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.provisioned = append(f.provisioned, req)
	if f.provisionErr != nil {
		return f.provisionErr(req)
	}
	return nil
}

func (f *fakePlugin) Destroy(_ context.Context, instanceID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.destroyed = append(f.destroyed, instanceID)
	return f.destroyErr
}

func (f *fakePlugin) HealthCheck(context.Context, string) (plugins.HealthStatus, error) {
	return plugins.HealthStatus{Healthy: true, Ready: true, CheckedAt: time.Now()}, nil
}

func (f *fakePlugin) Configure(context.Context, string, plugins.InstanceConfig) error { return nil }
func (f *fakePlugin) Rollout(context.Context, string, plugins.ImageRef) error         { return nil }
func (f *fakePlugin) Rollback(context.Context, string, plugins.ImageRef) error        { return nil }

func (f *fakePlugin) Logs(context.Context, string, plugins.LogOpts) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (f *fakePlugin) Restart(context.Context, string) error { return nil }
func (f *fakePlugin) Close() error                         { return nil }

func testRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	reg, err := registry.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Close() })
	return reg
}

func okConfigWriter(_ config.InjectParams) error { return nil }

func baseRequest(id string) Request {
	return Request{
		InstanceID:       id,
		AdminSecret:      "secret-admin",
		InstancePassword: "secret-pass",
		Image:            plugins.ImageRef{Repository: "ghcr.io/fox/runtime", Digest: "sha256:abc123"},
	}
}

func TestProvisionHappyPath(t *testing.T) {
	reg := testRegistry(t)
	plug := &fakePlugin{}
	dataRoot := t.TempDir()

	prov := New(Options{
		Registry:       reg,
		Plugin:         plug,
		ConfigWriter:   okConfigWriter,
		DataRoot:       dataRoot,
		PortRangeStart: 9100,
		MaxInstances:   2,
	})

	inst, err := prov.Provision(context.Background(), baseRequest("test-1"))
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}

	if inst.ID != "test-1" {
		t.Errorf("ID = %q, want %q", inst.ID, "test-1")
	}
	if inst.Port != 9100 {
		t.Errorf("Port = %d, want 9100", inst.Port)
	}
	if inst.Status != "running" {
		t.Errorf("Status = %q, want %q", inst.Status, "running")
	}
	wantDir := filepath.Join(dataRoot, "instances", "test-1")
	if inst.DataDir != wantDir {
		t.Errorf("DataDir = %q, want %q", inst.DataDir, wantDir)
	}
	if inst.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}

	regInst, err := reg.Get("test-1")
	if err != nil {
		t.Fatalf("registry.Get: %v", err)
	}
	if regInst.Status != "running" {
		t.Errorf("registry status = %q, want %q", regInst.Status, "running")
	}

	if len(plug.provisioned) != 1 {
		t.Fatalf("plugin.Provision called %d times, want 1", len(plug.provisioned))
	}
	if plug.provisioned[0].Port != 9100 {
		t.Errorf("plugin port = %d, want 9100", plug.provisioned[0].Port)
	}
}

func TestProvisionAllocatesSequentialPorts(t *testing.T) {
	reg := testRegistry(t)
	plug := &fakePlugin{}
	dataRoot := t.TempDir()

	prov := New(Options{
		Registry:       reg,
		Plugin:         plug,
		ConfigWriter:   okConfigWriter,
		DataRoot:       dataRoot,
		PortRangeStart: 9100,
		MaxInstances:   3,
	})

	inst1, err := prov.Provision(context.Background(), baseRequest("a"))
	if err != nil {
		t.Fatal(err)
	}
	inst2, err := prov.Provision(context.Background(), baseRequest("b"))
	if err != nil {
		t.Fatal(err)
	}

	if inst1.Port != 9100 {
		t.Errorf("first port = %d, want 9100", inst1.Port)
	}
	if inst2.Port != 9101 {
		t.Errorf("second port = %d, want 9101", inst2.Port)
	}
}

func TestProvisionFillsGaps(t *testing.T) {
	reg := testRegistry(t)
	plug := &fakePlugin{}
	dataRoot := t.TempDir()

	prov := New(Options{
		Registry:       reg,
		Plugin:         plug,
		ConfigWriter:   okConfigWriter,
		DataRoot:       dataRoot,
		PortRangeStart: 9100,
		MaxInstances:   5,
	})

	_, err := prov.Provision(context.Background(), baseRequest("a"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = prov.Provision(context.Background(), baseRequest("b"))
	if err != nil {
		t.Fatal(err)
	}

	if err := prov.Destroy(context.Background(), "a", true); err != nil {
		t.Fatal(err)
	}

	inst, err := prov.Provision(context.Background(), baseRequest("c"))
	if err != nil {
		t.Fatal(err)
	}
	if inst.Port != 9100 {
		t.Errorf("gap-fill port = %d, want 9100", inst.Port)
	}
}

func TestProvisionAndDestroy(t *testing.T) {
	reg := testRegistry(t)
	plug := &fakePlugin{}
	dataRoot := t.TempDir()

	prov := New(Options{
		Registry:       reg,
		Plugin:         plug,
		ConfigWriter:   okConfigWriter,
		DataRoot:       dataRoot,
		PortRangeStart: 9100,
	})

	_, err := prov.Provision(context.Background(), baseRequest("test-1"))
	if err != nil {
		t.Fatal(err)
	}

	if err := prov.Destroy(context.Background(), "test-1", false); err != nil {
		t.Fatal(err)
	}

	_, err = reg.Get("test-1")
	if !errors.Is(err, registry.ErrNotFound) {
		t.Errorf("expected ErrNotFound after destroy, got: %v", err)
	}

	if len(plug.destroyed) != 1 || plug.destroyed[0] != "test-1" {
		t.Errorf("plugin.Destroy calls = %v, want [test-1]", plug.destroyed)
	}
}

func TestDestroyWithRemoveData(t *testing.T) {
	reg := testRegistry(t)
	plug := &fakePlugin{}
	dataRoot := t.TempDir()

	prov := New(Options{
		Registry:       reg,
		Plugin:         plug,
		ConfigWriter:   okConfigWriter,
		DataRoot:       dataRoot,
		PortRangeStart: 9100,
	})

	_, err := prov.Provision(context.Background(), baseRequest("test-1"))
	if err != nil {
		t.Fatal(err)
	}

	dataDir := filepath.Join(dataRoot, "instances", "test-1")
	if _, err := os.Stat(dataDir); err != nil {
		t.Fatalf("data dir should exist before destroy: %v", err)
	}

	if err := prov.Destroy(context.Background(), "test-1", true); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Error("data dir should be removed after destroy with removeData=true")
	}
}

func TestDestroyNotFound(t *testing.T) {
	reg := testRegistry(t)
	plug := &fakePlugin{}

	prov := New(Options{
		Registry:       reg,
		Plugin:         plug,
		ConfigWriter:   okConfigWriter,
		DataRoot:       t.TempDir(),
		PortRangeStart: 9100,
	})

	err := prov.Destroy(context.Background(), "nonexistent", false)
	if err == nil {
		t.Fatal("expected error destroying nonexistent instance")
	}
	if !errors.Is(err, registry.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestCapReached(t *testing.T) {
	reg := testRegistry(t)
	plug := &fakePlugin{}
	dataRoot := t.TempDir()

	prov := New(Options{
		Registry:       reg,
		Plugin:         plug,
		ConfigWriter:   okConfigWriter,
		DataRoot:       dataRoot,
		PortRangeStart: 9100,
		MaxInstances:   2,
	})

	if _, err := prov.Provision(context.Background(), baseRequest("a")); err != nil {
		t.Fatal(err)
	}
	if _, err := prov.Provision(context.Background(), baseRequest("b")); err != nil {
		t.Fatal(err)
	}

	_, err := prov.Provision(context.Background(), baseRequest("c"))
	if !errors.Is(err, ErrCapReached) {
		t.Errorf("expected ErrCapReached, got: %v", err)
	}
}

func TestInstanceAlreadyExists(t *testing.T) {
	reg := testRegistry(t)
	plug := &fakePlugin{}
	dataRoot := t.TempDir()

	prov := New(Options{
		Registry:       reg,
		Plugin:         plug,
		ConfigWriter:   okConfigWriter,
		DataRoot:       dataRoot,
		PortRangeStart: 9100,
	})

	if _, err := prov.Provision(context.Background(), baseRequest("dup")); err != nil {
		t.Fatal(err)
	}

	_, err := prov.Provision(context.Background(), baseRequest("dup"))
	if err == nil {
		t.Fatal("expected error for duplicate instance ID")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want to contain 'already exists'", err)
	}
}

func TestValidateSecretsFails(t *testing.T) {
	reg := testRegistry(t)
	plug := &fakePlugin{}

	prov := New(Options{
		Registry:       reg,
		Plugin:         plug,
		ConfigWriter:   okConfigWriter,
		DataRoot:       t.TempDir(),
		PortRangeStart: 9100,
	})

	_, err := prov.Provision(context.Background(), Request{
		InstanceID:       "bad",
		AdminSecret:      "",
		InstancePassword: "",
		Image:            plugins.ImageRef{Repository: "r", Digest: "d"},
	})
	if err == nil {
		t.Fatal("expected error for empty secrets")
	}
}

func TestConcurrentProvision(t *testing.T) {
	reg := testRegistry(t)
	plug := &fakePlugin{}
	dataRoot := t.TempDir()

	const n = 10
	prov := New(Options{
		Registry:       reg,
		Plugin:         plug,
		ConfigWriter:   okConfigWriter,
		DataRoot:       dataRoot,
		PortRangeStart: 9100,
		MaxInstances:   n,
	})

	var wg sync.WaitGroup
	results := make([]*Instance, n)
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := baseRequest(fmt.Sprintf("inst-%d", idx))
			results[idx], errs[idx] = prov.Provision(context.Background(), req)
		}(i)
	}
	wg.Wait()

	ports := make(map[int]string)
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Errorf("goroutine %d: %v", i, errs[i])
			continue
		}
		if prev, dup := ports[results[i].Port]; dup {
			t.Errorf("port %d assigned to both %s and %s", results[i].Port, prev, results[i].ID)
		}
		ports[results[i].Port] = results[i].ID
	}

	if len(ports) != n {
		t.Errorf("expected %d unique ports, got %d", n, len(ports))
	}
}

func TestPartialFailureConfigWrite(t *testing.T) {
	reg := testRegistry(t)
	plug := &fakePlugin{}
	dataRoot := t.TempDir()

	failWriter := func(_ config.InjectParams) error {
		return errors.New("disk full")
	}

	prov := New(Options{
		Registry:       reg,
		Plugin:         plug,
		ConfigWriter:   failWriter,
		DataRoot:       dataRoot,
		PortRangeStart: 9100,
	})

	_, err := prov.Provision(context.Background(), baseRequest("fail-cfg"))
	if err == nil {
		t.Fatal("expected error from config writer")
	}

	_, regErr := reg.Get("fail-cfg")
	if !errors.Is(regErr, registry.ErrNotFound) {
		t.Error("registry entry should be cleaned up after config failure")
	}

	dataDir := filepath.Join(dataRoot, "instances", "fail-cfg")
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Error("data dir should be cleaned up after config failure")
	}

	if len(plug.destroyed) != 0 {
		t.Error("plugin.Destroy should not be called when config injection fails")
	}
}

func TestPartialFailurePlugin(t *testing.T) {
	reg := testRegistry(t)
	plug := &fakePlugin{
		provisionErr: func(_ plugins.ProvisionRequest) error {
			return errors.New("docker: connection refused")
		},
	}
	dataRoot := t.TempDir()

	prov := New(Options{
		Registry:       reg,
		Plugin:         plug,
		ConfigWriter:   okConfigWriter,
		DataRoot:       dataRoot,
		PortRangeStart: 9100,
	})

	_, err := prov.Provision(context.Background(), baseRequest("fail-plug"))
	if err == nil {
		t.Fatal("expected error from plugin")
	}

	_, regErr := reg.Get("fail-plug")
	if !errors.Is(regErr, registry.ErrNotFound) {
		t.Error("registry entry should be cleaned up after plugin failure")
	}

	dataDir := filepath.Join(dataRoot, "instances", "fail-plug")
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Error("data dir should be cleaned up after plugin failure")
	}

	if len(plug.destroyed) != 1 || plug.destroyed[0] != "fail-plug" {
		t.Errorf("plugin.Destroy calls = %v, want [fail-plug]", plug.destroyed)
	}
}

func TestSecretsCleanedAfterFailure(t *testing.T) {
	reg := testRegistry(t)
	dataRoot := t.TempDir()

	realWriter := config.Inject

	plug := &fakePlugin{
		provisionErr: func(_ plugins.ProvisionRequest) error {
			return errors.New("container create failed")
		},
	}

	prov := New(Options{
		Registry:       reg,
		Plugin:         plug,
		ConfigWriter:   realWriter,
		DataRoot:       dataRoot,
		PortRangeStart: 9100,
	})

	_, err := prov.Provision(context.Background(), baseRequest("secrets-test"))
	if err == nil {
		t.Fatal("expected error")
	}

	dataDir := filepath.Join(dataRoot, "instances", "secrets-test")
	for _, name := range []string{"hermes.env", "config.yaml", "settings.json"} {
		path := filepath.Join(dataDir, name)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s should not persist after failed provision", name)
		}
	}
}

func TestDefaultHealthInterval(t *testing.T) {
	if got := DefaultHealthInterval(0); got != 2*time.Second {
		t.Errorf("at 0s: got %v, want 2s", got)
	}
	if got := DefaultHealthInterval(29 * time.Second); got != 2*time.Second {
		t.Errorf("at 29s: got %v, want 2s", got)
	}
	if got := DefaultHealthInterval(30 * time.Second); got != 5*time.Second {
		t.Errorf("at 30s: got %v, want 5s", got)
	}
	if got := DefaultHealthInterval(60 * time.Second); got != 5*time.Second {
		t.Errorf("at 60s: got %v, want 5s", got)
	}
}

func TestProvisionWithSkillset(t *testing.T) {
	reg := testRegistry(t)
	plug := &fakePlugin{}
	dataRoot := t.TempDir()

	skillsetDir := t.TempDir()
	skillsetPath := filepath.Join(skillsetDir, "test-skillset.yaml")
	skillsetContent := `name: customer-support
version: "1.0.0"
contract_version: "1.0.0"
persona:
  system_prompt_file: prompt.txt
tools: []
data_sources: []
memory:
  provider: none
ui:
  branding:
    bot_name: Support Bot
    avatar: ""
  removals: []
capabilities: {}
`
	if err := os.WriteFile(skillsetPath, []byte(skillsetContent), 0o644); err != nil {
		t.Fatal(err)
	}

	var captured config.InjectParams
	prov := New(Options{
		Registry:       reg,
		Plugin:         plug,
		ConfigWriter:   func(p config.InjectParams) error { captured = p; return nil },
		DataRoot:       dataRoot,
		PortRangeStart: 9100,
		MaxInstances:   2,
	})

	req := baseRequest("fox-skill")
	req.SkillsetPath = skillsetPath
	req.PrincipalRole = "agent"

	inst, err := prov.Provision(context.Background(), req)
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}

	copiedPath := filepath.Join(inst.DataDir, "skillset.yaml")
	data, err := os.ReadFile(copiedPath)
	if err != nil {
		t.Fatalf("skillset not copied: %v", err)
	}
	if !strings.Contains(string(data), "customer-support") {
		t.Error("copied skillset missing expected content")
	}

	if captured.Config.SkillsetPath != "/data/skillset.yaml" {
		t.Errorf("injected SkillsetPath = %q, want /data/skillset.yaml", captured.Config.SkillsetPath)
	}
	if captured.Config.PrincipalRole != "agent" {
		t.Errorf("injected PrincipalRole = %q, want agent", captured.Config.PrincipalRole)
	}

	regInst, _ := reg.Get("fox-skill")
	if regInst.SkillsetName != "customer-support" {
		t.Errorf("registry SkillsetName = %q, want customer-support", regInst.SkillsetName)
	}
	if regInst.PrincipalRole != "agent" {
		t.Errorf("registry PrincipalRole = %q, want agent", regInst.PrincipalRole)
	}
}

func TestProvisionWithInvalidSkillset(t *testing.T) {
	reg := testRegistry(t)
	plug := &fakePlugin{}
	dataRoot := t.TempDir()

	skillsetPath := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(skillsetPath, []byte("not valid yaml: ["), 0o644); err != nil {
		t.Fatal(err)
	}

	prov := New(Options{
		Registry:       reg,
		Plugin:         plug,
		ConfigWriter:   okConfigWriter,
		DataRoot:       dataRoot,
		PortRangeStart: 9100,
		MaxInstances:   2,
	})

	req := baseRequest("fox-bad-skill")
	req.SkillsetPath = skillsetPath

	_, err := prov.Provision(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid skillset")
	}
	if !strings.Contains(err.Error(), "skillset") {
		t.Errorf("error should mention skillset: %v", err)
	}
}

func TestProvisionWithMissingSkillset(t *testing.T) {
	reg := testRegistry(t)
	plug := &fakePlugin{}
	dataRoot := t.TempDir()

	prov := New(Options{
		Registry:       reg,
		Plugin:         plug,
		ConfigWriter:   okConfigWriter,
		DataRoot:       dataRoot,
		PortRangeStart: 9100,
		MaxInstances:   2,
	})

	req := baseRequest("fox-missing")
	req.SkillsetPath = "/nonexistent/skillset.yaml"

	_, err := prov.Provision(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing skillset file")
	}
}

func TestInterfaceCompliance(t *testing.T) {
	var _ Provisioner = (*service)(nil)
}
