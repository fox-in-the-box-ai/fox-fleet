package main

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
)

type Config struct {
	Control   ControlSection   `toml:"control"`
	Docker    DockerSection    `toml:"docker"`
	Auth      AuthSection      `toml:"auth"`
	Instances InstancesSection `toml:"instances"`
}

type ControlSection struct {
	Listen            string `toml:"listen"`
	DataRoot          string `toml:"data_root"`
	HealthPollSeconds int    `toml:"health_poll_seconds"`
}

type DockerSection struct {
	Socket string `toml:"socket"`
	Image  string `toml:"image"`
}

type AuthSection struct {
	AdminSecret      string `toml:"admin_secret"`
	InstancePassword string `toml:"instance_password"`
}

type InstancesSection struct {
	PortStart    int `toml:"port_start"`
	MaxInstances int `toml:"max_instances"`
}

func LoadConfig(path string) (*Config, error) {
	var cfg Config
	md, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	if md.IsDefined("instances", "max_assistants") {
		return nil, fmt.Errorf("config: instances.max_assistants is not a valid key — use instances.max_instances (see PRODUCTS.md §4)")
	}

	applyDefaults(&cfg)

	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Control.Listen == "" {
		cfg.Control.Listen = "127.0.0.1:9090"
	}
	if cfg.Docker.Socket == "" {
		cfg.Docker.Socket = defaultDockerSocket()
	}
	if cfg.Instances.PortStart == 0 {
		cfg.Instances.PortStart = 8787
	}
	if cfg.Instances.MaxInstances == 0 {
		cfg.Instances.MaxInstances = 2
	}
	if cfg.Control.HealthPollSeconds == 0 {
		cfg.Control.HealthPollSeconds = 15
	}
}

func validateConfig(cfg *Config) error {
	var errs []string
	if cfg.Control.DataRoot == "" {
		errs = append(errs, "control.data_root is required")
	}
	if cfg.Docker.Image == "" {
		errs = append(errs, "docker.image is required")
	}
	if cfg.Auth.AdminSecret == "" {
		errs = append(errs, "auth.admin_secret must not be empty")
	}
	if cfg.Auth.InstancePassword == "" {
		errs = append(errs, "auth.instance_password must not be empty")
	}
	if len(errs) > 0 {
		return fmt.Errorf("config: validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

func defaultDockerSocket() string {
	if runtime.GOOS == "windows" {
		return "//./pipe/docker_engine"
	}
	return "/var/run/docker.sock"
}

func parseImageRef(s string) plugins.ImageRef {
	if i := strings.LastIndex(s, "@"); i > 0 {
		return plugins.ImageRef{
			Repository: s[:i],
			Digest:     s[i+1:],
		}
	}
	return plugins.ImageRef{Repository: s}
}
