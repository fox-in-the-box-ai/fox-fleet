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
)

var (
	ErrMissingAuthSecret       = errors.New("config: auth_secret must not be empty")
	ErrMissingInstancePassword = errors.New("config: instance_password must not be empty")
)

type InjectParams struct {
	DataDir          string
	InstancePassword string
	Config           plugins.InstanceConfig
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
	}{
		{"hermes.env", renderHermesEnv},
		{"config.yaml", renderConfigYAML},
		{"settings.json", renderSettingsJSON},
	}

	for _, w := range writers {
		data, err := w.fn(p)
		if err != nil {
			return fmt.Errorf("config: render %s: %w", w.name, err)
		}
		if err := writeIfChanged(filepath.Join(p.DataDir, w.name), data); err != nil {
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
	if p.Config.SkillsetPath != "" {
		env["FOX_SKILLSET_PATH"] = p.Config.SkillsetPath
	}
	for k, v := range p.Config.Env {
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
	var buf bytes.Buffer
	buf.WriteString("# Managed by fox-control — do not edit manually.\n")
	if p.Config.ProxyEndpoint != "" {
		fmt.Fprintf(&buf, "proxy_endpoint: %s\n", p.Config.ProxyEndpoint)
	}
	if p.Config.PrincipalRole != "" {
		fmt.Fprintf(&buf, "principal_role: %s\n", p.Config.PrincipalRole)
	}
	if len(p.Config.CapabilityFlags) > 0 {
		buf.WriteString("capabilities:\n")
		keys := make([]string, 0, len(p.Config.CapabilityFlags))
		for k := range p.Config.CapabilityFlags {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&buf, "  %s: %t\n", k, p.Config.CapabilityFlags[k])
		}
	}
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

func writeIfChanged(path string, data []byte) error {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
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
