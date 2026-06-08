package skillsets

import (
	"fmt"
	"regexp"
	"strings"
)

var semverRe = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

type Manifest struct {
	Name            string          `yaml:"name" json:"name"`
	Version         string          `yaml:"version" json:"version"`
	ContractVersion string          `yaml:"contract_version" json:"contract_version"`
	Persona         Persona         `yaml:"persona" json:"persona"`
	Tools           []Tool          `yaml:"tools" json:"tools"`
	DataSources     []DataSource    `yaml:"data_sources" json:"data_sources"`
	Memory          Memory          `yaml:"memory" json:"memory"`
	UI              UI              `yaml:"ui" json:"ui"`
	Capabilities    map[string]bool `yaml:"capabilities" json:"capabilities"`
}

type Persona struct {
	SystemPromptFile string `yaml:"system_prompt_file" json:"system_prompt_file"`
}

type Tool struct {
	Name string `yaml:"name" json:"name"`
	Type string `yaml:"type" json:"type"`
}

type DataSource struct {
	Binding   string `yaml:"binding" json:"binding"`
	QueryMode string `yaml:"query_mode" json:"query_mode"`
}

type Memory struct {
	Provider string            `yaml:"provider" json:"provider"`
	Config   map[string]string `yaml:"config" json:"config"`
}

type UI struct {
	Branding Branding `yaml:"branding" json:"branding"`
	Removals []string `yaml:"removals" json:"removals"`
}

type Branding struct {
	BotName string `yaml:"bot_name" json:"bot_name"`
	Avatar  string `yaml:"avatar" json:"avatar"`
}

func Validate(m *Manifest) error {
	var errs []string

	if m.Name == "" {
		errs = append(errs, "name is required")
	}
	if m.Version == "" {
		errs = append(errs, "version is required")
	} else if !semverRe.MatchString(m.Version) {
		errs = append(errs, fmt.Sprintf("version %q is not valid semver", m.Version))
	}
	if m.ContractVersion == "" {
		errs = append(errs, "contract_version is required")
	} else if !semverRe.MatchString(m.ContractVersion) {
		errs = append(errs, fmt.Sprintf("contract_version %q is not valid semver", m.ContractVersion))
	}

	for i, tool := range m.Tools {
		if tool.Name == "" {
			errs = append(errs, fmt.Sprintf("tools[%d].name is required", i))
		}
		if tool.Type == "" {
			errs = append(errs, fmt.Sprintf("tools[%d].type is required", i))
		} else if tool.Type != "builtin" && tool.Type != "custom" {
			errs = append(errs, fmt.Sprintf("tools[%d].type %q must be \"builtin\" or \"custom\"", i, tool.Type))
		}
	}

	for i, ds := range m.DataSources {
		if ds.Binding == "" {
			errs = append(errs, fmt.Sprintf("data_sources[%d].binding is required", i))
		}
		if ds.QueryMode == "" {
			errs = append(errs, fmt.Sprintf("data_sources[%d].query_mode is required", i))
		} else if ds.QueryMode != "rag" && ds.QueryMode != "function_call" {
			errs = append(errs, fmt.Sprintf("data_sources[%d].query_mode %q must be \"rag\" or \"function_call\"", i, ds.QueryMode))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("skillset validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}
