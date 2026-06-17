package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fox-control.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func validTOML() string {
	return `
[control]
listen = "127.0.0.1:9090"
data_root = "/tmp/fox-test"

[docker]
socket = "/var/run/docker.sock"
image = "ghcr.io/fox/runtime:stable"

[auth]
admin_secret = "test-secret-long-enough"
instance_password = "test-pass"

[instances]
port_start = 8787
max_instances = 2
`
}

func TestLoadConfig_Valid(t *testing.T) {
	cfg, err := LoadConfig(writeConfig(t, validTOML()))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Control.Listen != "127.0.0.1:9090" {
		t.Errorf("Listen = %q", cfg.Control.Listen)
	}
	if cfg.Control.DataRoot != "/tmp/fox-test" {
		t.Errorf("DataRoot = %q", cfg.Control.DataRoot)
	}
	if cfg.Docker.Image != "ghcr.io/fox/runtime:stable" {
		t.Errorf("Image = %q", cfg.Docker.Image)
	}
	if cfg.Auth.AdminSecret != "test-secret-long-enough" {
		t.Errorf("AdminSecret = %q", cfg.Auth.AdminSecret)
	}
	if cfg.Auth.InstancePassword != "test-pass" {
		t.Errorf("InstancePassword = %q", cfg.Auth.InstancePassword)
	}
	if cfg.Instances.PortStart != 8787 {
		t.Errorf("PortStart = %d", cfg.Instances.PortStart)
	}
	if cfg.Instances.MaxInstances != 2 {
		t.Errorf("MaxInstances = %d", cfg.Instances.MaxInstances)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	content := `
[control]
data_root = "/tmp/fox-test"

[docker]
image = "ghcr.io/fox/runtime:stable"

[auth]
admin_secret = "default-test-secret-ok"
instance_password = "p"
`
	cfg, err := LoadConfig(writeConfig(t, content))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Control.Listen != "127.0.0.1:9090" {
		t.Errorf("default Listen = %q, want 127.0.0.1:9090", cfg.Control.Listen)
	}
	if cfg.Docker.Socket != defaultDockerSocket() {
		t.Errorf("default Socket = %q, want %q", cfg.Docker.Socket, defaultDockerSocket())
	}
	if cfg.Instances.PortStart != 8787 {
		t.Errorf("default PortStart = %d, want 8787", cfg.Instances.PortStart)
	}
	if cfg.Instances.MaxInstances != 2 {
		t.Errorf("default MaxInstances = %d, want 2", cfg.Instances.MaxInstances)
	}
}

func TestLoadConfig_MissingAdminSecret(t *testing.T) {
	content := `
[control]
data_root = "/tmp/fox-test"

[docker]
image = "ghcr.io/fox/runtime:stable"

[auth]
instance_password = "p"
`
	_, err := LoadConfig(writeConfig(t, content))
	if err == nil {
		t.Fatal("expected error for missing admin_secret")
	}
	if !strings.Contains(err.Error(), "admin_secret") {
		t.Errorf("error = %q, want to mention admin_secret", err)
	}
}

func TestLoadConfig_MissingInstancePassword(t *testing.T) {
	content := `
[control]
data_root = "/tmp/fox-test"

[docker]
image = "ghcr.io/fox/runtime:stable"

[auth]
admin_secret = "s"
`
	_, err := LoadConfig(writeConfig(t, content))
	if err == nil {
		t.Fatal("expected error for missing instance_password")
	}
	if !strings.Contains(err.Error(), "instance_password") {
		t.Errorf("error = %q, want to mention instance_password", err)
	}
}

func TestLoadConfig_MissingDataRoot(t *testing.T) {
	content := `
[docker]
image = "ghcr.io/fox/runtime:stable"

[auth]
admin_secret = "s"
instance_password = "p"
`
	_, err := LoadConfig(writeConfig(t, content))
	if err == nil {
		t.Fatal("expected error for missing data_root")
	}
	if !strings.Contains(err.Error(), "data_root") {
		t.Errorf("error = %q, want to mention data_root", err)
	}
}

func TestLoadConfig_MissingImage(t *testing.T) {
	content := `
[control]
data_root = "/tmp/fox-test"

[auth]
admin_secret = "s"
instance_password = "p"
`
	_, err := LoadConfig(writeConfig(t, content))
	if err == nil {
		t.Fatal("expected error for missing image")
	}
	if !strings.Contains(err.Error(), "docker.image") {
		t.Errorf("error = %q, want to mention docker.image", err)
	}
}

func TestLoadConfig_MaxAssistantsRejected(t *testing.T) {
	content := `
[control]
data_root = "/tmp/fox-test"

[docker]
image = "ghcr.io/fox/runtime:stable"

[auth]
admin_secret = "s"
instance_password = "p"

[instances]
max_assistants = 5
`
	_, err := LoadConfig(writeConfig(t, content))
	if err == nil {
		t.Fatal("expected error for max_assistants")
	}
	if !strings.Contains(err.Error(), "max_assistants") {
		t.Errorf("error = %q, want to mention max_assistants", err)
	}
	if !strings.Contains(err.Error(), "PRODUCTS.md") {
		t.Errorf("error = %q, want to reference PRODUCTS.md", err)
	}
}

func TestLoadConfig_InvalidTOML(t *testing.T) {
	_, err := LoadConfig(writeConfig(t, "not valid {{{{ toml"))
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/fox-control.toml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadConfig_AllRequiredFieldsMissing(t *testing.T) {
	content := `
[control]
listen = "127.0.0.1:9090"
`
	_, err := LoadConfig(writeConfig(t, content))
	if err == nil {
		t.Fatal("expected error for multiple missing fields")
	}
	for _, field := range []string{"data_root", "docker.image", "admin_secret", "instance_password"} {
		if !strings.Contains(err.Error(), field) {
			t.Errorf("error = %q, want to mention %s", err, field)
		}
	}
}

func TestParseImageRef_WithDigest(t *testing.T) {
	ref := parseImageRef("ghcr.io/fox/runtime@sha256:abc123")
	if ref.Repository != "ghcr.io/fox/runtime" {
		t.Errorf("Repository = %q", ref.Repository)
	}
	if ref.Digest != "sha256:abc123" {
		t.Errorf("Digest = %q", ref.Digest)
	}
}

func TestParseImageRef_WithTag(t *testing.T) {
	ref := parseImageRef("ghcr.io/fox/runtime:stable")
	if ref.Repository != "ghcr.io/fox/runtime:stable" {
		t.Errorf("Repository = %q", ref.Repository)
	}
	if ref.Digest != "" {
		t.Errorf("Digest = %q, want empty", ref.Digest)
	}
}

func TestVersionCmd(t *testing.T) {
	old := [3]string{buildVersion, buildCommit, buildDate}
	t.Cleanup(func() { buildVersion, buildCommit, buildDate = old[0], old[1], old[2] })

	buildVersion = "1.0.0"
	buildCommit = "abc1234"
	buildDate = "2026-06-07"

	cmd := newVersionCmd()
	var buf strings.Builder
	cmd.SetOut(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "1.0.0") {
		t.Errorf("output = %q, want to contain version", out)
	}
	if !strings.Contains(out, "abc1234") {
		t.Errorf("output = %q, want to contain commit", out)
	}
}

func TestRootCmd_HasSubcommands(t *testing.T) {
	root := newRootCmd()

	want := map[string]bool{
		"serve": false, "provision": false, "destroy": false,
		"list": false, "version": false,
	}

	for _, cmd := range root.Commands() {
		if _, ok := want[cmd.Name()]; ok {
			want[cmd.Name()] = true
		}
	}

	for name, found := range want {
		if !found {
			t.Errorf("missing subcommand: %s", name)
		}
	}
}

func TestListCmd_EmptyRegistry(t *testing.T) {
	dir := t.TempDir()
	content := fmt.Sprintf(`
[control]
data_root = %q

[docker]
image = "test:latest"

[auth]
admin_secret = "list-empty-test-secret"
instance_password = "p"
`, dir)

	configFile := writeConfig(t, content)

	root := newRootCmd()
	var buf strings.Builder
	root.SetOut(&buf)
	root.SetArgs([]string{"--config", configFile, "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("list: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ID") {
		t.Error("expected header row in output")
	}
}

func TestProvisionCmd_RequiresID(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"provision"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --id is missing")
	}
}

func TestDestroyCmd_RequiresID(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"destroy"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --id is missing")
	}
}

func TestDestroyCmd_HasRemoveDataFlag(t *testing.T) {
	cmd := newDestroyCmd()
	flag := cmd.Flags().Lookup("remove-data")
	if flag == nil {
		t.Fatal("missing --remove-data flag")
	}
	if flag.DefValue != "false" {
		t.Errorf("--remove-data default = %q, want false", flag.DefValue)
	}
}

func TestLoadConfig_QdrantSection(t *testing.T) {
	content := validTOML() + `
[qdrant]
enabled = true
image = "qdrant/qdrant:v1.14.1"
http_port = 6333
grpc_port = 6334
data_dir = "/var/lib/fox-control/qdrant"
`
	cfg, err := LoadConfig(writeConfig(t, content))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.Qdrant.Enabled {
		t.Error("Qdrant.Enabled = false, want true")
	}
	if cfg.Qdrant.Image != "qdrant/qdrant:v1.14.1" {
		t.Errorf("Qdrant.Image = %q", cfg.Qdrant.Image)
	}
	if cfg.Qdrant.HTTPPort != 6333 {
		t.Errorf("Qdrant.HTTPPort = %d", cfg.Qdrant.HTTPPort)
	}
	if cfg.Qdrant.GRPCPort != 6334 {
		t.Errorf("Qdrant.GRPCPort = %d", cfg.Qdrant.GRPCPort)
	}
	if cfg.Qdrant.DataDir != "/var/lib/fox-control/qdrant" {
		t.Errorf("Qdrant.DataDir = %q", cfg.Qdrant.DataDir)
	}
}

func TestLoadConfig_QdrantSectionDefaults(t *testing.T) {
	cfg, err := LoadConfig(writeConfig(t, validTOML()))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Qdrant.Enabled {
		t.Error("Qdrant.Enabled should default to false")
	}
}

func TestLoadConfig_QdrantEnabledDefaultsDataDir(t *testing.T) {
	content := validTOML() + `
[qdrant]
enabled = true
image = "qdrant/qdrant:v1.14.1"
http_port = 6333
grpc_port = 6334
`
	cfg, err := LoadConfig(writeConfig(t, content))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	want := cfg.Control.DataRoot + "/qdrant"
	if cfg.Qdrant.DataDir != want {
		t.Errorf("Qdrant.DataDir = %q, want %q", cfg.Qdrant.DataDir, want)
	}
}

func TestLoadConfig_ValidationHardening(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name: "negative port_start",
			config: `
[control]
data_root = "/tmp/fox-test"
listen = "127.0.0.1:9090"
[docker]
image = "ghcr.io/fox/runtime:stable"
[auth]
admin_secret = "s"
instance_password = "p"
[instances]
port_start = -1
max_instances = 2
`,
			wantErr: "instances.port_start",
		},
		{
			name: "port_start exceeds 65535",
			config: `
[control]
data_root = "/tmp/fox-test"
listen = "127.0.0.1:9090"
[docker]
image = "ghcr.io/fox/runtime:stable"
[auth]
admin_secret = "s"
instance_password = "p"
[instances]
port_start = 70000
max_instances = 2
`,
			wantErr: "instances.port_start",
		},
		{
			name: "max_instances exceeds 1000",
			config: `
[control]
data_root = "/tmp/fox-test"
listen = "127.0.0.1:9090"
[docker]
image = "ghcr.io/fox/runtime:stable"
[auth]
admin_secret = "s"
instance_password = "p"
[instances]
max_instances = 1500
port_start = 8787
`,
			wantErr: "instances.max_instances",
		},
		{
			name: "port range overflows 65535",
			config: `
[control]
data_root = "/tmp/fox-test"
listen = "127.0.0.1:9090"
[docker]
image = "ghcr.io/fox/runtime:stable"
[auth]
admin_secret = "s"
instance_password = "p"
[instances]
port_start = 65000
max_instances = 600
`,
			wantErr: "exceeds port 65535",
		},
		{
			name: "unparseable control.listen",
			config: `
[control]
listen = "not-a-host-port"
data_root = "/tmp/fox-test"
[docker]
image = "ghcr.io/fox/runtime:stable"
[auth]
admin_secret = "s"
instance_password = "p"
`,
			wantErr: "control.listen",
		},
		{
			name: "negative health_poll_seconds",
			config: `
[control]
health_poll_seconds = -5
listen = "127.0.0.1:9090"
data_root = "/tmp/fox-test"
[docker]
image = "ghcr.io/fox/runtime:stable"
[auth]
admin_secret = "s"
instance_password = "p"
`,
			wantErr: "health_poll_seconds",
		},
		{
			name: "qdrant enabled missing image",
			config: validTOML() + `
[qdrant]
enabled = true
http_port = 6333
grpc_port = 6334
`,
			wantErr: "qdrant.image",
		},
		{
			name: "qdrant ports equal",
			config: validTOML() + `
[qdrant]
enabled = true
image = "qdrant:v1"
http_port = 6333
grpc_port = 6333
`,
			wantErr: "must differ",
		},
		{
			name: "qdrant port out of range",
			config: validTOML() + `
[qdrant]
enabled = true
image = "qdrant:v1"
http_port = 0
grpc_port = 6334
`,
			wantErr: "qdrant.http_port",
		},
		{
			name: "port collision control vs instances",
			config: `
[control]
listen = "127.0.0.1:8787"
data_root = "/tmp/fox-test"
[docker]
image = "ghcr.io/fox/runtime:stable"
[auth]
admin_secret = "collision-test-secret"
instance_password = "p"
[instances]
port_start = 8787
max_instances = 2
`,
			wantErr: "collides with instances.port_start",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadConfig(writeConfig(t, tt.config))
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfig_CloudDefaults(t *testing.T) {
	cfg, err := LoadConfig(writeConfig(t, validTOML()))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Cloud.Enabled {
		t.Error("Cloud.Enabled should default to false")
	}
	if cfg.Cloud.SessionTTL != 86400 {
		t.Errorf("Cloud.SessionTTL = %d, want 86400", cfg.Cloud.SessionTTL)
	}
	if cfg.Cloud.LoginRateLimit != 5 {
		t.Errorf("Cloud.LoginRateLimit = %d, want 5", cfg.Cloud.LoginRateLimit)
	}
	if cfg.Cloud.CookieName != "fox_cloud_session" {
		t.Errorf("Cloud.CookieName = %q, want %q", cfg.Cloud.CookieName, "fox_cloud_session")
	}
}

func TestLoadConfig_CloudEnabled(t *testing.T) {
	content := validTOML() + `
[cloud]
enabled = true
domain = "cloud.example.com"
`
	cfg, err := LoadConfig(writeConfig(t, content))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.Cloud.Enabled {
		t.Error("Cloud.Enabled = false, want true")
	}
	if cfg.Cloud.Domain != "cloud.example.com" {
		t.Errorf("Cloud.Domain = %q, want %q", cfg.Cloud.Domain, "cloud.example.com")
	}
}

func TestLoadConfig_CloudEnabledMissingDomain(t *testing.T) {
	content := validTOML() + `
[cloud]
enabled = true
`
	_, err := LoadConfig(writeConfig(t, content))
	if err == nil {
		t.Fatal("expected error for cloud.enabled without domain")
	}
	if !strings.Contains(err.Error(), "cloud.domain") {
		t.Errorf("error = %q, want to mention cloud.domain", err)
	}
}

func TestLoadConfig_CloudDisabledNoDomainRequired(t *testing.T) {
	content := validTOML() + `
[cloud]
enabled = false
`
	_, err := LoadConfig(writeConfig(t, content))
	if err != nil {
		t.Fatalf("LoadConfig with cloud.enabled=false should not require domain: %v", err)
	}
}
