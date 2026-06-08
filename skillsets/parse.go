package skillsets

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("skillset: parse YAML: %w", err)
	}
	if err := Validate(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

func LoadFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("skillset: read %s: %w", path, err)
	}
	return Parse(data)
}
