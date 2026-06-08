package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
)

func validParams(t *testing.T) InjectParams {
	t.Helper()
	return InjectParams{
		DataDir:          t.TempDir(),
		InstancePassword: "inst-pass-123",
		Config: plugins.InstanceConfig{
			AuthSecret:    "admin-secret-abc",
			ProxyEndpoint: "https://proxy.example.com",
			PrincipalRole: "user",
			CapabilityFlags: map[string]bool{
				"chat":   true,
				"search": false,
			},
			Env: map[string]string{
				"CUSTOM_VAR": "custom-value",
			},
		},
	}
}

func TestInjectCreatesAllFiles(t *testing.T) {
	p := validParams(t)
	if err := Inject(p); err != nil {
		t.Fatalf("Inject: %v", err)
	}
	for _, name := range []string{"hermes.env", "config.yaml", "settings.json", "tools.json"} {
		path := filepath.Join(p.DataDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s not created: %v", name, err)
		}
	}
}

func TestHermesEnvContents(t *testing.T) {
	p := validParams(t)
	if err := Inject(p); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(p.DataDir, "hermes.env"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	required := []string{
		"FOX_PLANE_AUTH_SECRET=admin-secret-abc",
		"HERMES_WEBUI_PASSWORD=inst-pass-123",
		"CUSTOM_VAR=custom-value",
	}
	for _, line := range required {
		if !strings.Contains(content, line) {
			t.Errorf("hermes.env missing %q\ngot:\n%s", line, content)
		}
	}
}

func TestHermesEnvSorted(t *testing.T) {
	p := validParams(t)
	if err := Inject(p); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(p.DataDir, "hermes.env"))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i] < lines[i-1] {
			t.Errorf("hermes.env not sorted: %q before %q", lines[i-1], lines[i])
		}
	}
}

func TestConfigYAMLContents(t *testing.T) {
	p := validParams(t)
	if err := Inject(p); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(p.DataDir, "config.yaml"))
	content := string(data)
	if !strings.Contains(content, "proxy_endpoint: https://proxy.example.com") {
		t.Errorf("config.yaml missing proxy_endpoint, got:\n%s", content)
	}
	if !strings.Contains(content, "principal_role: user") {
		t.Errorf("config.yaml missing principal_role, got:\n%s", content)
	}
	if !strings.Contains(content, "chat: true") {
		t.Errorf("config.yaml missing capability chat, got:\n%s", content)
	}
}

func TestSettingsJSONContents(t *testing.T) {
	p := validParams(t)
	if err := Inject(p); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(p.DataDir, "settings.json"))
	content := string(data)
	if !strings.Contains(content, `"proxy_endpoint"`) {
		t.Errorf("settings.json missing proxy_endpoint, got:\n%s", content)
	}
	if !strings.Contains(content, `"capabilities"`) {
		t.Errorf("settings.json missing capabilities, got:\n%s", content)
	}
}

func TestInjectFailsOnMissingAuthSecret(t *testing.T) {
	p := validParams(t)
	p.Config.AuthSecret = ""
	err := Inject(p)
	if err != ErrMissingAuthSecret {
		t.Errorf("Inject(empty auth_secret) = %v, want ErrMissingAuthSecret", err)
	}
}

func TestInjectFailsOnMissingInstancePassword(t *testing.T) {
	p := validParams(t)
	p.InstancePassword = ""
	err := Inject(p)
	if err != ErrMissingInstancePassword {
		t.Errorf("Inject(empty instance_password) = %v, want ErrMissingInstancePassword", err)
	}
}

func TestInjectIdempotent(t *testing.T) {
	p := validParams(t)
	if err := Inject(p); err != nil {
		t.Fatal(err)
	}

	envPath := filepath.Join(p.DataDir, "hermes.env")
	info1, _ := os.Stat(envPath)

	if err := Inject(p); err != nil {
		t.Fatal(err)
	}

	info2, _ := os.Stat(envPath)
	if !info1.ModTime().Equal(info2.ModTime()) {
		t.Error("idempotent Inject modified hermes.env when content unchanged")
	}
}

func TestInjectOverwritesOnChange(t *testing.T) {
	p := validParams(t)
	if err := Inject(p); err != nil {
		t.Fatal(err)
	}

	data1, _ := os.ReadFile(filepath.Join(p.DataDir, "hermes.env"))

	p.Config.AuthSecret = "new-secret"
	if err := Inject(p); err != nil {
		t.Fatal(err)
	}

	data2, _ := os.ReadFile(filepath.Join(p.DataDir, "hermes.env"))
	if string(data1) == string(data2) {
		t.Error("Inject did not update hermes.env after config change")
	}
	if !strings.Contains(string(data2), "FOX_PLANE_AUTH_SECRET=new-secret") {
		t.Error("hermes.env does not contain updated secret")
	}
}

func TestInjectCreatesDataDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "data")
	p := validParams(t)
	p.DataDir = dir
	if err := Inject(p); err != nil {
		t.Fatalf("Inject with nested dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "hermes.env")); err != nil {
		t.Error("hermes.env not created in nested dir")
	}
}

func TestValidateSecrets(t *testing.T) {
	if err := ValidateSecrets("secret", "password"); err != nil {
		t.Errorf("ValidateSecrets with valid inputs: %v", err)
	}
	if err := ValidateSecrets("", "password"); err == nil {
		t.Error("ValidateSecrets should fail with empty admin_secret")
	}
	if err := ValidateSecrets("secret", ""); err == nil {
		t.Error("ValidateSecrets should fail with empty instance_password")
	}
	err := ValidateSecrets("", "")
	if err == nil {
		t.Error("ValidateSecrets should fail with both empty")
	}
	if err != nil && !strings.Contains(err.Error(), "admin_secret") {
		t.Errorf("error should mention admin_secret: %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "instance_password") {
		t.Errorf("error should mention instance_password: %v", err)
	}
}

func TestToolsJSONEmpty(t *testing.T) {
	p := validParams(t)
	if err := Inject(p); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(p.DataDir, "tools.json"))
	content := string(data)
	if !strings.Contains(content, `"tools": []`) {
		t.Errorf("tools.json without DataPlaneURL should have empty tools, got:\n%s", content)
	}
}

func TestToolsJSONWithDataPlane(t *testing.T) {
	p := validParams(t)
	p.Config.DataPlaneURL = "http://127.0.0.1:9091"
	if err := Inject(p); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(p.DataDir, "tools.json"))
	content := string(data)
	for _, want := range []string{
		`"name": "knowledge_query"`,
		`"url": "http://127.0.0.1:9091/v1/query"`,
		`"method": "POST"`,
		`"header": "X-Fox-Auth"`,
		`"env": "FOX_PLANE_AUTH_SECRET"`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("tools.json missing %q, got:\n%s", want, content)
		}
	}
}

func TestInjectMinimalConfig(t *testing.T) {
	p := InjectParams{
		DataDir:          t.TempDir(),
		InstancePassword: "pass",
		Config: plugins.InstanceConfig{
			AuthSecret: "secret",
		},
	}
	if err := Inject(p); err != nil {
		t.Fatalf("Inject with minimal config: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(p.DataDir, "hermes.env"))
	if !strings.Contains(string(data), "FOX_PLANE_AUTH_SECRET=secret") {
		t.Error("minimal config missing auth secret in hermes.env")
	}
}
