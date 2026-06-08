package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
)

var blockedEnvKeys = map[string]bool{
	"FOX_PLANE_AUTH_SECRET": true,
	"FOX_PROXY_ENDPOINT":   true,
	"FOX_DATA_PLANE_URL":   true,
	"FOX_DATA_PLANE_TOKEN": true,
	"FOX_SKILLSET_PATH":    true,
	"HERMES_WEBUI_PASSWORD": true,
	"PATH":                 true,
	"HOME":                 true,
	"LD_PRELOAD":           true,
	"LD_LIBRARY_PATH":      true,
}

const (
	containerPrefix = "fox-"
	healthPollDelay = 2 * time.Second
	healthPollMax   = 30
	stopTimeout     = 30
	internalPort    = "8080/tcp"
)

type Plugin struct {
	cli *client.Client
}

func New() (*Plugin, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("docker: create client: %w", err)
	}
	return &Plugin{cli: cli}, nil
}

func NewWithClient(cli *client.Client) *Plugin {
	return &Plugin{cli: cli}
}

func (p *Plugin) Provision(ctx context.Context, req plugins.ProvisionRequest) error {
	cname := containerPrefix + req.InstanceID

	env := []string{
		fmt.Sprintf("FOX_PLANE_AUTH_SECRET=%s", req.Config.AuthSecret),
	}
	if req.Config.ProxyEndpoint != "" {
		env = append(env, fmt.Sprintf("FOX_PROXY_ENDPOINT=%s", req.Config.ProxyEndpoint))
	}
	if req.Config.DataPlaneURL != "" {
		env = append(env, fmt.Sprintf("FOX_DATA_PLANE_URL=%s", req.Config.DataPlaneURL))
	}
	if req.Config.SkillsetPath != "" {
		env = append(env, fmt.Sprintf("FOX_SKILLSET_PATH=%s", req.Config.SkillsetPath))
	}
	for k, v := range req.Config.Env {
		upper := strings.ToUpper(k)
		if blockedEnvKeys[upper] {
			return fmt.Errorf("docker: env key %q is reserved and cannot be overridden", k)
		}
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	hostPort := fmt.Sprintf("%d", req.Port)
	portBinding := nat.PortMap{
		nat.Port(internalPort): []nat.PortBinding{
			{HostIP: "127.0.0.1", HostPort: hostPort},
		},
	}

	imageRef := refString(req.Image)

	resp, err := p.cli.ContainerCreate(ctx,
		&container.Config{
			Image: imageRef,
			Env:   env,
			ExposedPorts: nat.PortSet{
				nat.Port(internalPort): struct{}{},
			},
			Labels: map[string]string{
				"fox.instance-id":  req.InstanceID,
				"fox.managed-by":   "fox-control",
				"fox.image-digest": req.Image.Digest,
			},
		},
		&container.HostConfig{
			PortBindings: portBinding,
			Binds:        []string{req.DataDir + ":/data"},
			RestartPolicy: container.RestartPolicy{
				Name: container.RestartPolicyUnlessStopped,
			},
		},
		nil, nil, cname,
	)
	if err != nil {
		return fmt.Errorf("docker: create container %s: %w", cname, err)
	}

	if err := p.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("docker: start container %s: %w", cname, err)
	}

	if err := p.pollHealth(ctx, req.Port); err != nil {
		return fmt.Errorf("docker: health poll %s: %w", cname, err)
	}

	return nil
}

func (p *Plugin) HealthCheck(ctx context.Context, instanceID string) (plugins.HealthStatus, error) {
	cname := containerPrefix + instanceID

	info, err := p.cli.ContainerInspect(ctx, cname)
	if err != nil {
		return plugins.HealthStatus{}, fmt.Errorf("docker: inspect %s: %w", cname, err)
	}

	if !info.State.Running {
		return plugins.HealthStatus{
			Healthy:   false,
			Ready:     false,
			CheckedAt: time.Now().UTC(),
		}, nil
	}

	port := extractHostPort(info)
	if port == 0 {
		return plugins.HealthStatus{
			Healthy:   false,
			Ready:     false,
			CheckedAt: time.Now().UTC(),
		}, nil
	}

	healthy := httpProbe(ctx, port, "/health")
	ready := httpProbe(ctx, port, "/readyz")

	return plugins.HealthStatus{
		Healthy:   healthy,
		Ready:     ready,
		Checks:    map[string]bool{"http_health": healthy, "http_readyz": ready},
		CheckedAt: time.Now().UTC(),
	}, nil
}

func (p *Plugin) Configure(ctx context.Context, instanceID string, cfg plugins.InstanceConfig) error {
	cname := containerPrefix + instanceID

	_, err := p.cli.ContainerInspect(ctx, cname)
	if err != nil {
		return fmt.Errorf("docker: inspect %s: %w", cname, err)
	}

	return nil
}

func (p *Plugin) Rollout(ctx context.Context, instanceID string, target plugins.ImageRef) error {
	cname := containerPrefix + instanceID

	info, err := p.cli.ContainerInspect(ctx, cname)
	if err != nil {
		return fmt.Errorf("docker: inspect %s for rollout: %w", cname, err)
	}

	imageRef := refString(target)
	pullReader, err := p.cli.ImagePull(ctx, imageRef, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("docker: pull %s: %w", imageRef, err)
	}
	_, _ = io.Copy(io.Discard, pullReader)
	pullReader.Close()

	timeout := stopTimeout
	if err := p.cli.ContainerStop(ctx, cname, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("docker: stop %s for rollout: %w", cname, err)
	}

	if err := p.cli.ContainerRemove(ctx, cname, container.RemoveOptions{}); err != nil {
		return fmt.Errorf("docker: remove %s for rollout: %w", cname, err)
	}

	newConfig := *info.Config
	newConfig.Image = imageRef
	newConfig.Labels["fox.image-digest"] = target.Digest

	resp, err := p.cli.ContainerCreate(ctx,
		&newConfig,
		info.HostConfig,
		nil, nil, cname,
	)
	if err != nil {
		return fmt.Errorf("docker: recreate %s: %w", cname, err)
	}

	if err := p.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("docker: restart %s: %w", cname, err)
	}

	port := extractHostPort(info)
	if port > 0 {
		if err := p.pollHealth(ctx, port); err != nil {
			return fmt.Errorf("docker: rollout health %s: %w", cname, err)
		}
	}

	return nil
}

func (p *Plugin) Rollback(ctx context.Context, instanceID string, previous plugins.ImageRef) error {
	return p.Rollout(ctx, instanceID, previous)
}

func (p *Plugin) Destroy(ctx context.Context, instanceID string) error {
	cname := containerPrefix + instanceID

	timeout := stopTimeout
	err := p.cli.ContainerStop(ctx, cname, container.StopOptions{Timeout: &timeout})
	if err != nil && !client.IsErrNotFound(err) {
		return fmt.Errorf("docker: stop %s: %w", cname, err)
	}

	err = p.cli.ContainerRemove(ctx, cname, container.RemoveOptions{})
	if err != nil && !client.IsErrNotFound(err) {
		return fmt.Errorf("docker: remove %s: %w", cname, err)
	}

	return nil
}

func (p *Plugin) Logs(ctx context.Context, instanceID string, opts plugins.LogOpts) (io.ReadCloser, error) {
	cname := containerPrefix + instanceID

	logOpts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     opts.Follow,
	}
	if opts.Tail > 0 {
		logOpts.Tail = fmt.Sprintf("%d", opts.Tail)
	}

	reader, err := p.cli.ContainerLogs(ctx, cname, logOpts)
	if err != nil {
		return nil, fmt.Errorf("docker: logs %s: %w", cname, err)
	}
	return reader, nil
}

func (p *Plugin) Restart(ctx context.Context, instanceID string) error {
	cname := containerPrefix + instanceID
	timeout := stopTimeout
	if err := p.cli.ContainerStop(ctx, cname, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("docker: stop %s for restart: %w", cname, err)
	}
	return p.cli.ContainerStart(ctx, cname, container.StartOptions{})
}

func (p *Plugin) Stats(ctx context.Context, instanceID string) (plugins.ContainerStats, error) {
	cname := containerPrefix + instanceID

	resp, err := p.cli.ContainerStatsOneShot(ctx, cname)
	if err != nil {
		return plugins.ContainerStats{}, fmt.Errorf("docker: stats %s: %w", cname, err)
	}
	defer resp.Body.Close()

	var statsJSON container.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&statsJSON); err != nil {
		return plugins.ContainerStats{}, fmt.Errorf("docker: stats %s: decode: %w", cname, err)
	}

	cpuPercent := calculateCPUPercent(statsJSON)

	var rxBytes, txBytes uint64
	for _, netStats := range statsJSON.Networks {
		rxBytes += netStats.RxBytes
		txBytes += netStats.TxBytes
	}

	return plugins.ContainerStats{
		CPUPercent:  cpuPercent,
		MemoryUsed:  statsJSON.MemoryStats.Usage,
		MemoryLimit: statsJSON.MemoryStats.Limit,
		NetRxBytes:  rxBytes,
		NetTxBytes:  txBytes,
	}, nil
}

func calculateCPUPercent(s container.StatsResponse) float64 {
	cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage) - float64(s.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(s.CPUStats.SystemUsage) - float64(s.PreCPUStats.SystemUsage)
	if systemDelta <= 0 || cpuDelta < 0 {
		return 0
	}
	return (cpuDelta / systemDelta) * float64(s.CPUStats.OnlineCPUs) * 100.0
}

func (p *Plugin) Close() error {
	return p.cli.Close()
}

func (p *Plugin) pollHealth(ctx context.Context, port int) error {
	for i := 0; i < healthPollMax; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(healthPollDelay):
		}
		if httpProbe(ctx, port, "/health") {
			return nil
		}
	}
	return fmt.Errorf("health check timed out after %d attempts", healthPollMax)
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

func extractHostPort(info types.ContainerJSON) int {
	bindings, ok := info.HostConfig.PortBindings[nat.Port(internalPort)]
	if !ok || len(bindings) == 0 {
		return 0
	}
	var port int
	_, _ = fmt.Sscanf(bindings[0].HostPort, "%d", &port)
	return port
}

func refString(ref plugins.ImageRef) string {
	if ref.Digest != "" {
		return ref.Repository + "@" + ref.Digest
	}
	return ref.Repository
}

var _ plugins.DeploymentPlugin = (*Plugin)(nil)
