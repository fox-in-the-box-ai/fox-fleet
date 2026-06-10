package sut

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const (
	internalPort   = "8787/tcp"
	healthTimeout  = 120 * time.Second
	healthInterval = 2 * time.Second
	probeTimeout   = 3 * time.Second
)

var nameCounter atomic.Int64

func Start(ctx context.Context, cli *client.Client, cfg Config) (*Handle, error) {
	seq := nameCounter.Add(1)
	name := fmt.Sprintf("fox-conf-%d-%d", os.Getpid(), seq)

	dataDir, err := os.MkdirTemp("", "fox-conf-*")
	if err != nil {
		return nil, fmt.Errorf("sut: create data dir: %w", err)
	}

	env := buildEnv(cfg)

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: cfg.Image,
			Env:   env,
			ExposedPorts: nat.PortSet{
				nat.Port(internalPort): {},
			},
		},
		&container.HostConfig{
			PortBindings: nat.PortMap{
				nat.Port(internalPort): {{HostIP: "127.0.0.1", HostPort: "0"}},
			},
			Binds:  []string{dataDir + ":/data"},
			CapAdd: []string{"NET_ADMIN"},
			ExtraHosts: []string{
				"host.docker.internal:host-gateway",
			},
		},
		nil, nil, name,
	)
	if err != nil {
		os.RemoveAll(dataDir)
		return nil, fmt.Errorf("sut: create container %s: %w", name, err)
	}

	h := &Handle{
		ContainerID: resp.ID,
		Name:        name,
		AuthSecret:  cfg.AuthSecret,
		Password:    cfg.Password,
		dataDir:     dataDir,
		cli:         cli,
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		h.Cleanup(ctx)
		return nil, fmt.Errorf("sut: start container %s: %w", name, err)
	}

	port, err := resolvePort(ctx, cli, resp.ID)
	if err != nil {
		h.Cleanup(ctx)
		return nil, fmt.Errorf("sut: resolve port %s: %w", name, err)
	}
	h.Port = port
	h.BaseURL = fmt.Sprintf("http://127.0.0.1:%d", port)

	if cfg.Mode != BootInvariant {
		if err := waitHealthy(ctx, port); err != nil {
			h.Cleanup(ctx)
			return nil, fmt.Errorf("sut: %s not healthy: %w", name, err)
		}
		if err := markOnboardingComplete(dataDir); err != nil {
			h.Cleanup(ctx)
			return nil, fmt.Errorf("sut: %s write onboarding marker: %w", name, err)
		}
	}

	return h, nil
}

func buildEnv(cfg Config) []string {
	var env []string
	if cfg.AuthSecret != "" {
		env = append(env, "FOX_PLANE_AUTH_SECRET="+cfg.AuthSecret)
	}
	if cfg.Password != "" {
		env = append(env, "HERMES_WEBUI_PASSWORD="+cfg.Password)
	}
	for k, v := range cfg.ExtraEnv {
		env = append(env, k+"="+v)
	}
	return env
}

func resolvePort(ctx context.Context, cli *client.Client, containerID string) (int, error) {
	info, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return 0, err
	}
	bindings, ok := info.NetworkSettings.Ports[nat.Port(internalPort)]
	if !ok || len(bindings) == 0 {
		return 0, fmt.Errorf("no port binding for %s", internalPort)
	}
	port, err := strconv.Atoi(bindings[0].HostPort)
	if err != nil {
		return 0, fmt.Errorf("parse host port %q: %w", bindings[0].HostPort, err)
	}
	return port, nil
}

func waitHealthy(ctx context.Context, port int) error {
	deadline := time.After(healthTimeout)
	for {
		select {
		case <-deadline:
			return fmt.Errorf("health check timed out after %s", healthTimeout)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(healthInterval):
			if probe(ctx, port, "/health") {
				return nil
			}
		}
	}
}

func markOnboardingComplete(dataDir string) error {
	dir := filepath.Join(dataDir, "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "onboarding.json"), []byte(`{"completed":true}`), 0o644)
}

func probe(ctx context.Context, port int, path string) bool {
	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	c := &http.Client{Timeout: probeTimeout}
	resp, err := c.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
