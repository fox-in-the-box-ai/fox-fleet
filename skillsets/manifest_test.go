package skillsets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validManifestYAML = `
name: fox-assistant
version: "1.0.0"
contract_version: "2.0.0"

persona:
  system_prompt_file: SOUL.md

tools:
  - name: web_search
    type: builtin
  - name: file_upload
    type: builtin

data_sources:
  - binding: customer_knowledge
    query_mode: rag

memory:
  provider: mem0_oss
  config:
    collection: "{instance_id}"

ui:
  branding:
    bot_name: Fox
    avatar: fox-logo.svg
  removals:
    - admin/settings/ollama
    - admin/settings/tailscale

capabilities:
  local_fallback: true
  data_plane_access: true
`

func TestParse_ValidManifest(t *testing.T) {
	m, err := Parse([]byte(validManifestYAML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Name != "fox-assistant" {
		t.Errorf("Name = %q", m.Name)
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version = %q", m.Version)
	}
	if m.ContractVersion != "2.0.0" {
		t.Errorf("ContractVersion = %q", m.ContractVersion)
	}
	if m.Persona.SystemPromptFile != "SOUL.md" {
		t.Errorf("Persona.SystemPromptFile = %q", m.Persona.SystemPromptFile)
	}
	if len(m.Tools) != 2 {
		t.Fatalf("len(Tools) = %d, want 2", len(m.Tools))
	}
	if m.Tools[0].Name != "web_search" || m.Tools[0].Type != "builtin" {
		t.Errorf("Tools[0] = %+v", m.Tools[0])
	}
	if len(m.DataSources) != 1 {
		t.Fatalf("len(DataSources) = %d, want 1", len(m.DataSources))
	}
	if m.DataSources[0].Binding != "customer_knowledge" {
		t.Errorf("DataSources[0].Binding = %q", m.DataSources[0].Binding)
	}
	if m.DataSources[0].QueryMode != "rag" {
		t.Errorf("DataSources[0].QueryMode = %q", m.DataSources[0].QueryMode)
	}
	if m.Memory.Provider != "mem0_oss" {
		t.Errorf("Memory.Provider = %q", m.Memory.Provider)
	}
	if m.Memory.Config["collection"] != "{instance_id}" {
		t.Errorf("Memory.Config[collection] = %q", m.Memory.Config["collection"])
	}
	if m.UI.Branding.BotName != "Fox" {
		t.Errorf("UI.Branding.BotName = %q", m.UI.Branding.BotName)
	}
	if len(m.UI.Removals) != 2 {
		t.Errorf("len(UI.Removals) = %d, want 2", len(m.UI.Removals))
	}
	if !m.Capabilities["local_fallback"] {
		t.Error("Capabilities[local_fallback] = false, want true")
	}
	if !m.Capabilities["data_plane_access"] {
		t.Error("Capabilities[data_plane_access] = false, want true")
	}
}

func TestParse_MissingName(t *testing.T) {
	yaml := `
version: "1.0.0"
contract_version: "2.0.0"
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error = %q, want to mention 'name is required'", err)
	}
}

func TestParse_MissingVersion(t *testing.T) {
	yaml := `
name: test
contract_version: "2.0.0"
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing version")
	}
	if !strings.Contains(err.Error(), "version is required") {
		t.Errorf("error = %q, want to mention 'version is required'", err)
	}
}

func TestParse_InvalidSemver(t *testing.T) {
	yaml := `
name: test
version: "not-semver"
contract_version: "2.0.0"
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for invalid semver")
	}
	if !strings.Contains(err.Error(), "not valid semver") {
		t.Errorf("error = %q, want to mention 'not valid semver'", err)
	}
}

func TestParse_InvalidContractVersion(t *testing.T) {
	yaml := `
name: test
version: "1.0.0"
contract_version: "bad"
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for invalid contract_version")
	}
	if !strings.Contains(err.Error(), "contract_version") {
		t.Errorf("error = %q, want to mention 'contract_version'", err)
	}
}

func TestParse_InvalidToolType(t *testing.T) {
	yaml := `
name: test
version: "1.0.0"
contract_version: "2.0.0"
tools:
  - name: my_tool
    type: invalid
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for invalid tool type")
	}
	if !strings.Contains(err.Error(), "builtin") {
		t.Errorf("error = %q, want to mention valid types", err)
	}
}

func TestParse_MissingToolName(t *testing.T) {
	yaml := `
name: test
version: "1.0.0"
contract_version: "2.0.0"
tools:
  - type: builtin
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing tool name")
	}
	if !strings.Contains(err.Error(), "tools[0].name") {
		t.Errorf("error = %q, want to mention 'tools[0].name'", err)
	}
}

func TestParse_InvalidQueryMode(t *testing.T) {
	yaml := `
name: test
version: "1.0.0"
contract_version: "2.0.0"
data_sources:
  - binding: kb
    query_mode: unknown
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for invalid query_mode")
	}
	if !strings.Contains(err.Error(), "query_mode") {
		t.Errorf("error = %q, want to mention 'query_mode'", err)
	}
}

func TestParse_MissingDataSourceBinding(t *testing.T) {
	yaml := `
name: test
version: "1.0.0"
contract_version: "2.0.0"
data_sources:
  - query_mode: rag
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing binding")
	}
	if !strings.Contains(err.Error(), "binding is required") {
		t.Errorf("error = %q, want to mention 'binding is required'", err)
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	_, err := Parse([]byte("{{{{ not yaml"))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParse_PrereleaseSemver(t *testing.T) {
	yaml := `
name: test
version: "1.0.0-alpha.1"
contract_version: "2.0.0-rc.1"
`
	m, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Version != "1.0.0-alpha.1" {
		t.Errorf("Version = %q", m.Version)
	}
}

func TestParse_MinimalManifest(t *testing.T) {
	yaml := `
name: minimal
version: "0.1.0"
contract_version: "2.0.0"
`
	m, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Name != "minimal" {
		t.Errorf("Name = %q", m.Name)
	}
	if len(m.Tools) != 0 {
		t.Errorf("len(Tools) = %d, want 0", len(m.Tools))
	}
}

func TestLoadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skillset.yaml")
	if err := os.WriteFile(path, []byte(validManifestYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if m.Name != "fox-assistant" {
		t.Errorf("Name = %q", m.Name)
	}
}

func TestLoadFile_NotFound(t *testing.T) {
	_, err := LoadFile("/nonexistent/skillset.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestParse_MultipleErrors(t *testing.T) {
	yaml := `
version: "bad"
contract_version: "also-bad"
tools:
  - type: nope
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "name is required") {
		t.Errorf("error = %q, want to mention name", errStr)
	}
	if !strings.Contains(errStr, "version") {
		t.Errorf("error = %q, want to mention version", errStr)
	}
	if !strings.Contains(errStr, "tools[0].name") {
		t.Errorf("error = %q, want to mention tools", errStr)
	}
}

func TestValidate_FunctionCallQueryMode(t *testing.T) {
	m := &Manifest{
		Name:            "test",
		Version:         "1.0.0",
		ContractVersion: "2.0.0",
		DataSources: []DataSource{
			{Binding: "kb", QueryMode: "function_call"},
		},
	}
	if err := Validate(m); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestValidate_CustomToolType(t *testing.T) {
	m := &Manifest{
		Name:            "test",
		Version:         "1.0.0",
		ContractVersion: "2.0.0",
		Tools: []Tool{
			{Name: "my_tool", Type: "custom"},
		},
	}
	if err := Validate(m); err != nil {
		t.Errorf("Validate: %v", err)
	}
}
