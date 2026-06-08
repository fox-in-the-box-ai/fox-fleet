package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
	"gopkg.in/yaml.v3"
)

var (
	ErrMissingAuthSecret       = errors.New("config: auth_secret must not be empty")
	ErrMissingInstancePassword = errors.New("config: instance_password must not be empty")
)

type InjectParams struct {
	DataDir          string
	InstancePassword string
	Config           plugins.InstanceConfig
	QueryToken       string
}

func Inject(p InjectParams) error {
	if p.Config.AuthSecret == "" {
		return ErrMissingAuthSecret
	}
	if p.InstancePassword == "" {
		return ErrMissingInstancePassword
	}

	writers := []struct {
		name string
		fn   func(InjectParams) ([]byte, error)
		perm os.FileMode
	}{
		{"hermes.env", renderHermesEnv, 0o600},
		{"config.yaml", renderConfigYAML, 0o644},
		{"settings.json", renderSettingsJSON, 0o644},
		{"tools.json", renderToolsJSON, 0o600},
	}

	for _, w := range writers {
		data, err := w.fn(p)
		if err != nil {
			return fmt.Errorf("config: render %s: %w", w.name, err)
		}
		if err := writeIfChanged(filepath.Join(p.DataDir, w.name), data, w.perm); err != nil {
			return fmt.Errorf("config: write %s: %w", w.name, err)
		}
	}
	return nil
}

func renderHermesEnv(p InjectParams) ([]byte, error) {
	env := map[string]string{
		"FOX_PLANE_AUTH_SECRET": p.Config.AuthSecret,
		"HERMES_WEBUI_PASSWORD": p.InstancePassword,
	}
	if p.Config.DataPlaneURL != "" {
		env["FOX_DATA_PLANE_URL"] = p.Config.DataPlaneURL
	}
	if p.QueryToken != "" {
		env["FOX_DATA_PLANE_TOKEN"] = p.QueryToken
	}
	if p.Config.SkillsetPath != "" {
		env["FOX_SKILLSET_PATH"] = p.Config.SkillsetPath
	}
	for k, v := range p.Config.Env {
		upper := strings.ToUpper(k)
		if upper == "FOX_PLANE_AUTH_SECRET" || upper == "HERMES_WEBUI_PASSWORD" ||
			upper == "FOX_DATA_PLANE_URL" || upper == "FOX_DATA_PLANE_TOKEN" ||
			upper == "FOX_SKILLSET_PATH" || upper == "PATH" ||
			upper == "HOME" || upper == "LD_PRELOAD" || upper == "LD_LIBRARY_PATH" {
			return nil, fmt.Errorf("env key %q is reserved and cannot be overridden", k)
		}
		env[k] = v
	}

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	for _, k := range keys {
		fmt.Fprintf(&buf, "%s=%s\n", k, env[k])
	}
	return buf.Bytes(), nil
}

func renderConfigYAML(p InjectParams) ([]byte, error) {
	cfg := make(map[string]any)
	if p.Config.ProxyEndpoint != "" {
		cfg["proxy_endpoint"] = p.Config.ProxyEndpoint
	}
	if p.Config.PrincipalRole != "" {
		cfg["principal_role"] = p.Config.PrincipalRole
	}
	if len(p.Config.CapabilityFlags) > 0 {
		cfg["capabilities"] = p.Config.CapabilityFlags
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.WriteString("# Managed by fox-control — do not edit manually.\n")
	buf.Write(data)
	return buf.Bytes(), nil
}

func renderSettingsJSON(p InjectParams) ([]byte, error) {
	settings := map[string]any{}
	if p.Config.ProxyEndpoint != "" {
		settings["proxy_endpoint"] = p.Config.ProxyEndpoint
	}
	if len(p.Config.CapabilityFlags) > 0 {
		settings["capabilities"] = p.Config.CapabilityFlags
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	return data, nil
}

func renderToolsJSON(p InjectParams) ([]byte, error) {
	type toolParam struct {
		Type        string `json:"type"`
		Required    bool   `json:"required,omitempty"`
		Default     any    `json:"default,omitempty"`
		Description string `json:"description"`
	}
	type toolAuth struct {
		Header string `json:"header"`
		Env    string `json:"env"`
	}
	type toolDef struct {
		Name        string               `json:"name"`
		Description string               `json:"description"`
		URL         string               `json:"url"`
		Method      string               `json:"method"`
		Auth        *toolAuth            `json:"auth,omitempty"`
		Parameters  map[string]toolParam `json:"parameters"`
	}
	type manifest struct {
		Tools []toolDef `json:"tools"`
	}

	m := manifest{Tools: []toolDef{}}
	if p.Config.DataPlaneURL != "" {
		m.Tools = append(m.Tools, toolDef{
			Name:        "knowledge_query",
			Description: "Search the knowledge base for relevant information",
			URL:         p.Config.DataPlaneURL + "/v1/query",
			Method:      "POST",
			Auth:        &toolAuth{Header: "X-Fox-Auth", Env: "FOX_DATA_PLANE_TOKEN"},
			Parameters: map[string]toolParam{
				"query":     {Type: "string", Required: true, Description: "Search query text"},
				"top_k":     {Type: "integer", Default: 5, Description: "Maximum number of results to return"},
				"source_id": {Type: "string", Description: "Filter results by source ID"},
			},
		})
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	return data, nil
}

func writeIfChanged(path string, data []byte, perm os.FileMode) error {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

func ValidateSecrets(authSecret, instancePassword string) error {
	var errs []string
	if authSecret == "" {
		errs = append(errs, "admin_secret is empty")
	}
	if instancePassword == "" {
		errs = append(errs, "instance_password is empty")
	}
	if len(errs) > 0 {
		return fmt.Errorf("config: refusing to start: %s", strings.Join(errs, "; "))
	}
	return nil
}
