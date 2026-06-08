package qdrant

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const (
	containerName  = "fox-qdrant"
	defaultImage   = "qdrant/qdrant:v1.14.0"
	httpPort       = "6333/tcp"
	grpcPort       = "6334/tcp"
	healthPath     = "/healthz"
	healthPollMax  = 30
	healthInterval = 2 * time.Second
	stopTimeout    = 30
)

var ErrMissingDataDir = errors.New("qdrant: data_dir is required for persistent storage")

type Config struct {
	Image    string
	HTTPPort int
	GRPCPort int
	DataDir  string
}

func (c *Config) applyDefaults() {
	if c.Image == "" {
		c.Image = defaultImage
	}
	if c.HTTPPort == 0 {
		c.HTTPPort = 6333
	}
	if c.GRPCPort == 0 {
		c.GRPCPort = 6334
	}
}

func (c *Config) validate() error {
	if c.DataDir == "" {
		return ErrMissingDataDir
	}
	return nil
}

type Manager struct {
	cli *client.Client
	cfg Config
}

func NewManager(cli *client.Client, cfg Config) (*Manager, error) {
	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &Manager{cli: cli, cfg: cfg}, nil
}

func (m *Manager) Provision(ctx context.Context) error {
	info, err := m.cli.ContainerInspect(ctx, containerName)
	if err == nil {
		if info.State.Running {
			return m.pollHealth(ctx)
		}
		if err := m.cli.ContainerStart(ctx, info.ID, container.StartOptions{}); err != nil {
			return fmt.Errorf("qdrant: start existing container: %w", err)
		}
		return m.pollHealth(ctx)
	}

	if !client.IsErrNotFound(err) {
		return fmt.Errorf("qdrant: inspect container: %w", err)
	}

	m.ensureImage(ctx)

	portBindings := nat.PortMap{
		nat.Port(httpPort): []nat.PortBinding{
			{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", m.cfg.HTTPPort)},
		},
		nat.Port(grpcPort): []nat.PortBinding{
			{HostIP: "127.0.0.1", HostPort: fmt.Sprintf("%d", m.cfg.GRPCPort)},
		},
	}

	resp, err := m.cli.ContainerCreate(ctx,
		&container.Config{
			Image: m.cfg.Image,
			ExposedPorts: nat.PortSet{
				nat.Port(httpPort): struct{}{},
				nat.Port(grpcPort): struct{}{},
			},
			Labels: map[string]string{
				"fox.managed-by": "fox-control",
				"fox.component":  "qdrant",
			},
		},
		&container.HostConfig{
			PortBindings: portBindings,
			Binds:        []string{m.cfg.DataDir + ":/qdrant/storage"},
			RestartPolicy: container.RestartPolicy{
				Name: container.RestartPolicyUnlessStopped,
			},
		},
		nil, nil, containerName,
	)
	if err != nil {
		return fmt.Errorf("qdrant: create container: %w", err)
	}

	if err := m.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("qdrant: start container: %w", err)
	}

	return m.pollHealth(ctx)
}

func (m *Manager) HealthCheck(ctx context.Context) (bool, error) {
	info, err := m.cli.ContainerInspect(ctx, containerName)
	if err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("qdrant: inspect container: %w", err)
	}
	if !info.State.Running {
		return false, nil
	}
	return httpProbe(ctx, m.cfg.HTTPPort, healthPath), nil
}

func (m *Manager) Destroy(ctx context.Context) error {
	timeout := stopTimeout
	err := m.cli.ContainerStop(ctx, containerName, container.StopOptions{Timeout: &timeout})
	if err != nil && !client.IsErrNotFound(err) {
		return fmt.Errorf("qdrant: stop container: %w", err)
	}

	err = m.cli.ContainerRemove(ctx, containerName, container.RemoveOptions{})
	if err != nil && !client.IsErrNotFound(err) {
		return fmt.Errorf("qdrant: remove container: %w", err)
	}

	return nil
}

func (m *Manager) HTTPURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", m.cfg.HTTPPort)
}

func (m *Manager) GRPCURL() string {
	return fmt.Sprintf("127.0.0.1:%d", m.cfg.GRPCPort)
}

func (m *Manager) ensureImage(ctx context.Context) {
	_, _, err := m.cli.ImageInspectWithRaw(ctx, m.cfg.Image)
	if err == nil {
		return
	}
	pullReader, pullErr := m.cli.ImagePull(ctx, m.cfg.Image, image.PullOptions{})
	if pullErr != nil {
		return
	}
	_, _ = io.Copy(io.Discard, pullReader)
	pullReader.Close()
}

func (m *Manager) pollHealth(ctx context.Context) error {
	for i := 0; i < healthPollMax; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(healthInterval):
		}
		if httpProbe(ctx, m.cfg.HTTPPort, healthPath) {
			return nil
		}
	}
	return fmt.Errorf("qdrant: health check timed out after %d attempts", healthPollMax)
}

func httpProbe(ctx context.Context, port int, path string) bool {
	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	httpClient := &http.Client{Timeout: 3 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
