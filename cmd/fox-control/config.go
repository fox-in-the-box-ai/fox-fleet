package main

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
)

type Config struct {
	Control     ControlSection     `toml:"control"`
	Docker      DockerSection      `toml:"docker"`
	Auth        AuthSection        `toml:"auth"`
	Instances   InstancesSection   `toml:"instances"`
	Qdrant      QdrantSection      `toml:"qdrant"`
	DataPlane   DataPlaneSection   `toml:"data_plane"`
	Embedding   EmbeddingSection   `toml:"embedding"`
	TLS         TLSSection         `toml:"tls"`
	Webhooks    []WebhookConfig    `toml:"webhooks"`
	RateLimit   RateLimitSection   `toml:"rate_limit"`
	AutoRestart AutoRestartSection `toml:"auto_restart"`
}

type AutoRestartSection struct {
	Enabled         bool `toml:"enabled"`
	Threshold       int  `toml:"threshold"`
	CooldownSeconds int  `toml:"cooldown_seconds"`
}

type QdrantSection struct {
	Enabled  bool   `toml:"enabled"`
	Image    string `toml:"image"`
	HTTPPort int    `toml:"http_port"`
	GRPCPort int    `toml:"grpc_port"`
	DataDir  string `toml:"data_dir"`
}

type DataPlaneSection struct {
	Enabled    bool   `toml:"enabled"`
	Listen     string `toml:"listen"`
	Collection string `toml:"collection"`
	VectorSize int    `toml:"vector_size"`
}

type EmbeddingSection struct {
	BaseURL string `toml:"base_url"`
	APIKey  string `toml:"api_key"`
	Model   string `toml:"model"`
}

type ControlSection struct {
	Listen              string `toml:"listen"`
	DataRoot            string `toml:"data_root"`
	HealthPollSeconds   int    `toml:"health_poll_seconds"`
	SessionTokenTTLSecs int    `toml:"session_token_ttl_seconds"`
	LogFormat           string `toml:"log_format"`
	LogLevel            string `toml:"log_level"`
	MetricsEnabled      *bool  `toml:"metrics_enabled"`
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
	PortStart       int    `toml:"port_start"`
	MaxInstances    int    `toml:"max_instances"`
	DefaultSkillset string `toml:"default_skillset"`
	DefaultRole     string `toml:"default_role"`
}

type TLSSection struct {
	CertFile string `toml:"cert_file"`
	KeyFile  string `toml:"key_file"`
}

type WebhookConfig struct {
	URL    string   `toml:"url"`
	Events []string `toml:"events"`
	Secret string   `toml:"secret"`
}

type RateLimitSection struct {
	RequestsPerMinute  int `toml:"requests_per_minute"`
	ProvisionPerMinute int `toml:"provision_per_minute"`
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
	applyEnvOverrides(&cfg)

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
	if cfg.Control.SessionTokenTTLSecs == 0 {
		cfg.Control.SessionTokenTTLSecs = 600
	}
	if cfg.Control.LogFormat == "" {
		cfg.Control.LogFormat = "text"
	}
	if cfg.Control.LogLevel == "" {
		cfg.Control.LogLevel = "info"
	}
	if cfg.AutoRestart.Threshold == 0 {
		cfg.AutoRestart.Threshold = 3
	}
	if cfg.AutoRestart.CooldownSeconds == 0 {
		cfg.AutoRestart.CooldownSeconds = 300
	}
	if cfg.DataPlane.Listen == "" {
		cfg.DataPlane.Listen = "127.0.0.1:9091"
	}
	if cfg.DataPlane.Collection == "" {
		cfg.DataPlane.Collection = "fox-knowledge"
	}
	if cfg.DataPlane.VectorSize == 0 {
		cfg.DataPlane.VectorSize = 1536
	}
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("FOX_ADMIN_SECRET"); v != "" {
		cfg.Auth.AdminSecret = v
	}
	if v := os.Getenv("FOX_INSTANCE_PASSWORD"); v != "" {
		cfg.Auth.InstancePassword = v
	}
}

func validateConfig(cfg *Config) error {
	var errs []string
	if cfg.Control.DataRoot == "" {
		errs = append(errs, "control.data_root is required — set it to a writable directory path")
	}
	if cfg.Docker.Image == "" {
		errs = append(errs, "docker.image is required — set it to a Fox container image reference")
	}
	if cfg.Auth.AdminSecret == "" {
		errs = append(errs, "auth.admin_secret must not be empty — set it in config or via FOX_ADMIN_SECRET env var")
	} else if len(cfg.Auth.AdminSecret) < 16 {
		errs = append(errs, "auth.admin_secret must be at least 16 characters — use 'fox-control generate-secret' to create one")
	}
	if cfg.Auth.InstancePassword == "" {
		errs = append(errs, "auth.instance_password must not be empty — set it in config or via FOX_INSTANCE_PASSWORD env var")
	}
	if _, _, err := net.SplitHostPort(cfg.Control.Listen); err != nil {
		errs = append(errs, fmt.Sprintf("control.listen %q is not a valid host:port address", cfg.Control.Listen))
	}
	switch cfg.Control.LogFormat {
	case "text", "json":
	default:
		errs = append(errs, fmt.Sprintf("control.log_format must be \"text\" or \"json\", got %q", cfg.Control.LogFormat))
	}
	switch cfg.Control.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Sprintf("control.log_level must be one of debug, info, warn, error — got %q", cfg.Control.LogLevel))
	}
	if (cfg.TLS.CertFile == "") != (cfg.TLS.KeyFile == "") {
		errs = append(errs, "tls.cert_file and tls.key_file must both be set or both be empty")
	}
	if cfg.Control.HealthPollSeconds < 1 || cfg.Control.HealthPollSeconds > 3600 {
		errs = append(errs, fmt.Sprintf("control.health_poll_seconds must be between 1 and 3600, got %d", cfg.Control.HealthPollSeconds))
	}
	if cfg.Control.SessionTokenTTLSecs < 60 || cfg.Control.SessionTokenTTLSecs > 3600 {
		errs = append(errs, fmt.Sprintf("control.session_token_ttl_seconds must be between 60 and 3600, got %d", cfg.Control.SessionTokenTTLSecs))
	}
	if cfg.Instances.PortStart < 1 || cfg.Instances.PortStart > 65535 {
		errs = append(errs, fmt.Sprintf("instances.port_start must be between 1 and 65535, got %d", cfg.Instances.PortStart))
	}
	if cfg.Instances.MaxInstances < 1 || cfg.Instances.MaxInstances > 1000 {
		errs = append(errs, fmt.Sprintf("instances.max_instances must be between 1 and 1000, got %d", cfg.Instances.MaxInstances))
	}
	if cfg.Instances.PortStart > 0 && cfg.Instances.MaxInstances > 0 &&
		cfg.Instances.PortStart+cfg.Instances.MaxInstances-1 > 65535 {
		errs = append(errs, fmt.Sprintf("instances.port_start %d + max_instances %d exceeds port 65535",
			cfg.Instances.PortStart, cfg.Instances.MaxInstances))
	}
	if cfg.Qdrant.Enabled {
		if cfg.Qdrant.DataDir == "" && cfg.Control.DataRoot != "" {
			cfg.Qdrant.DataDir = cfg.Control.DataRoot + "/qdrant"
		}
		if cfg.Qdrant.DataDir == "" {
			errs = append(errs, "qdrant.data_dir is required when qdrant is enabled — set it or ensure control.data_root is set")
		}
		if cfg.Qdrant.Image == "" {
			errs = append(errs, "qdrant.image is required when qdrant is enabled — set it to a Qdrant container image reference")
		}
		if cfg.Qdrant.HTTPPort < 1 || cfg.Qdrant.HTTPPort > 65535 {
			errs = append(errs, fmt.Sprintf("qdrant.http_port must be between 1 and 65535, got %d", cfg.Qdrant.HTTPPort))
		}
		if cfg.Qdrant.GRPCPort < 1 || cfg.Qdrant.GRPCPort > 65535 {
			errs = append(errs, fmt.Sprintf("qdrant.grpc_port must be between 1 and 65535, got %d", cfg.Qdrant.GRPCPort))
		}
		if cfg.Qdrant.HTTPPort > 0 && cfg.Qdrant.GRPCPort > 0 && cfg.Qdrant.HTTPPort == cfg.Qdrant.GRPCPort {
			errs = append(errs, fmt.Sprintf("qdrant.http_port and qdrant.grpc_port must differ, both are %d", cfg.Qdrant.HTTPPort))
		}
	}
	if cfg.DataPlane.Enabled {
		if _, _, err := net.SplitHostPort(cfg.DataPlane.Listen); err != nil {
			errs = append(errs, fmt.Sprintf("data_plane.listen %q is not a valid host:port address", cfg.DataPlane.Listen))
		}
		if cfg.DataPlane.VectorSize < 1 || cfg.DataPlane.VectorSize > 65536 {
			errs = append(errs, fmt.Sprintf("data_plane.vector_size must be between 1 and 65536, got %d", cfg.DataPlane.VectorSize))
		}
		if cfg.Embedding.BaseURL == "" {
			errs = append(errs, "embedding.base_url is required when data_plane is enabled — set it to an OpenAI-compatible embedding API URL")
		}
		if cfg.Embedding.Model == "" {
			errs = append(errs, "embedding.model is required when data_plane is enabled — set it to the embedding model name")
		}
		if !cfg.Qdrant.Enabled {
			errs = append(errs, "qdrant must be enabled when data_plane is enabled — set qdrant.enabled = true")
		}
	}
	if len(errs) == 0 {
		errs = append(errs, checkPortCollisions(cfg)...)
	}
	if len(errs) > 0 {
		return fmt.Errorf("config: validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

func checkPortCollisions(cfg *Config) []string {
	type portEntry struct {
		port int
		key  string
	}
	var entries []portEntry

	if _, portStr, err := net.SplitHostPort(cfg.Control.Listen); err == nil {
		if p := parsePort(portStr); p > 0 {
			entries = append(entries, portEntry{p, "control.listen"})
		}
	}

	if cfg.Qdrant.Enabled {
		if cfg.Qdrant.HTTPPort > 0 {
			entries = append(entries, portEntry{cfg.Qdrant.HTTPPort, "qdrant.http_port"})
		}
		if cfg.Qdrant.GRPCPort > 0 {
			entries = append(entries, portEntry{cfg.Qdrant.GRPCPort, "qdrant.grpc_port"})
		}
	}

	if cfg.DataPlane.Enabled {
		if _, portStr, err := net.SplitHostPort(cfg.DataPlane.Listen); err == nil {
			if p := parsePort(portStr); p > 0 {
				entries = append(entries, portEntry{p, "data_plane.listen"})
			}
		}
	}

	instStart := cfg.Instances.PortStart
	instEnd := instStart + cfg.Instances.MaxInstances

	var errs []string
	for _, e := range entries {
		if e.port >= instStart && e.port < instEnd {
			errs = append(errs, fmt.Sprintf("%s port %d collides with instances.port_start range [%d, %d)",
				e.key, e.port, instStart, instEnd))
		}
	}
	return errs
}

func parsePort(s string) int {
	var p int
	if _, err := fmt.Sscanf(s, "%d", &p); err != nil {
		return 0
	}
	return p
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
