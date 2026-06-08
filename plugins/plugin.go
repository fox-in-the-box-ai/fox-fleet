package plugins

import (
	"context"
	"io"
	"time"
)

// DeploymentPlugin is the contract every deployment target must implement.
// The Docker plugin is the v0.1 reference; K8s follows in fox-fleet-enterprise.
type DeploymentPlugin interface {
	Provision(ctx context.Context, req ProvisionRequest) error
	HealthCheck(ctx context.Context, instanceID string) (HealthStatus, error)
	Configure(ctx context.Context, instanceID string, cfg InstanceConfig) error
	Rollout(ctx context.Context, instanceID string, target ImageRef) error
	Rollback(ctx context.Context, instanceID string, previous ImageRef) error
	Destroy(ctx context.Context, instanceID string) error
	Logs(ctx context.Context, instanceID string, opts LogOpts) (io.ReadCloser, error)
	Restart(ctx context.Context, instanceID string) error
	Stats(ctx context.Context, instanceID string) (ContainerStats, error)
}

// ImageRef identifies a container image by digest (never tag).
type ImageRef struct {
	Repository string `json:"repository"`
	Digest     string `json:"digest"`
}

// ProvisionRequest contains everything needed to create a new instance.
type ProvisionRequest struct {
	InstanceID string         `json:"instance_id"`
	Image      ImageRef       `json:"image"`
	Config     InstanceConfig `json:"config"`
	Port       int            `json:"port"`
	DataDir    string         `json:"data_dir"`
}

// InstanceConfig holds per-instance configuration injected before boot.
type InstanceConfig struct {
	AuthSecret      string            `json:"auth_secret"`
	ProxyEndpoint   string            `json:"proxy_endpoint,omitempty"`
	CapabilityFlags map[string]bool   `json:"capability_flags,omitempty"`
	Env             map[string]string `json:"env,omitempty"`
	SkillsetPath    string            `json:"skillset_path,omitempty"`
	DataPlaneURL    string            `json:"data_plane_url,omitempty"`
	PrincipalRole   string            `json:"principal_role,omitempty"`
}

// HealthStatus reports the health and readiness of an instance.
type HealthStatus struct {
	Healthy   bool            `json:"healthy"`
	Ready     bool            `json:"ready"`
	Checks    map[string]bool `json:"checks,omitempty"`
	CheckedAt time.Time       `json:"checked_at"`
}

// ContainerStats holds point-in-time resource usage for a container.
type ContainerStats struct {
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryUsed  uint64  `json:"memory_used"`
	MemoryLimit uint64  `json:"memory_limit"`
	NetRxBytes  uint64  `json:"net_rx_bytes"`
	NetTxBytes  uint64  `json:"net_tx_bytes"`
}

// LogOpts configures log retrieval.
type LogOpts struct {
	Tail   int  `json:"tail"`
	Follow bool `json:"follow"`
}
