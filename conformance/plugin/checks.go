package plugin

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/conformance/runtime/report"
	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
	"github.com/fox-in-the-box-ai/fox-fleet/plugins/docker"
)

func check01Provision(ctx context.Context, plug *docker.Plugin, imageRef plugins.ImageRef, dataDir string) report.Result {
	start := time.Now()

	provCtx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()

	err := plug.Provision(provCtx, plugins.ProvisionRequest{
		InstanceID: testInstanceID,
		Image:      imageRef,
		Config: plugins.InstanceConfig{
			AuthSecret: testAuthSecret,
		},
		Port:    testPort,
		DataDir: dataDir,
	})
	if err != nil {
		return report.Result{
			Number:   1,
			Name:     "Provision valid config",
			Status:   report.Fail,
			Duration: time.Since(start),
			Detail:   fmt.Sprintf("Provision failed: %v", err),
		}
	}

	if !httpProbe(ctx, testPort) {
		return report.Result{
			Number:   1,
			Name:     "Provision valid config",
			Status:   report.Fail,
			Duration: time.Since(start),
			Detail:   "Instance not reachable after provision",
		}
	}

	return report.Result{
		Number:   1,
		Name:     "Provision valid config",
		Status:   report.Pass,
		Duration: time.Since(start),
	}
}

func check02HealthCheck(ctx context.Context, plug *docker.Plugin) report.Result {
	start := time.Now()

	status, err := plug.HealthCheck(ctx, testInstanceID)
	if err != nil {
		return report.Result{
			Number:   2,
			Name:     "HealthCheck provisioned instance",
			Status:   report.Fail,
			Duration: time.Since(start),
			Detail:   fmt.Sprintf("HealthCheck error: %v", err),
		}
	}

	if !status.Healthy {
		return report.Result{
			Number:   2,
			Name:     "HealthCheck provisioned instance",
			Status:   report.Fail,
			Duration: time.Since(start),
			Detail:   "HealthCheck returned Healthy=false",
		}
	}

	return report.Result{
		Number:   2,
		Name:     "HealthCheck provisioned instance",
		Status:   report.Pass,
		Duration: time.Since(start),
	}
}

func check03Configure(ctx context.Context, plug *docker.Plugin) report.Result {
	start := time.Now()

	err := plug.Configure(ctx, testInstanceID, plugins.InstanceConfig{
		AuthSecret:    testAuthSecret,
		ProxyEndpoint: "http://example.com:8080",
	})
	if err != nil {
		return report.Result{
			Number:   3,
			Name:     "Configure running instance",
			Status:   report.Fail,
			Duration: time.Since(start),
			Detail:   fmt.Sprintf("Configure error: %v", err),
		}
	}

	return report.Result{
		Number:   3,
		Name:     "Configure running instance",
		Status:   report.Pass,
		Duration: time.Since(start),
	}
}

func check04Rollout(ctx context.Context, plug *docker.Plugin, imageRef plugins.ImageRef) report.Result {
	start := time.Now()

	rolloutCtx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()

	err := plug.Rollout(rolloutCtx, testInstanceID, imageRef)
	if err != nil {
		return report.Result{
			Number:   4,
			Name:     "Rollout to target image",
			Status:   report.Fail,
			Duration: time.Since(start),
			Detail:   fmt.Sprintf("Rollout error: %v", err),
		}
	}

	status, err := plug.HealthCheck(ctx, testInstanceID)
	if err != nil || !status.Healthy {
		return report.Result{
			Number:   4,
			Name:     "Rollout to target image",
			Status:   report.Fail,
			Duration: time.Since(start),
			Detail:   "Instance unhealthy after rollout",
		}
	}

	return report.Result{
		Number:   4,
		Name:     "Rollout to target image",
		Status:   report.Pass,
		Duration: time.Since(start),
	}
}

func check05Rollback(ctx context.Context, plug *docker.Plugin, imageRef plugins.ImageRef) report.Result {
	start := time.Now()

	rollbackCtx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()

	err := plug.Rollback(rollbackCtx, testInstanceID, imageRef)
	if err != nil {
		return report.Result{
			Number:   5,
			Name:     "Rollback to previous image",
			Status:   report.Fail,
			Duration: time.Since(start),
			Detail:   fmt.Sprintf("Rollback error: %v", err),
		}
	}

	status, err := plug.HealthCheck(ctx, testInstanceID)
	if err != nil || !status.Healthy {
		return report.Result{
			Number:   5,
			Name:     "Rollback to previous image",
			Status:   report.Fail,
			Duration: time.Since(start),
			Detail:   "Instance unhealthy after rollback",
		}
	}

	return report.Result{
		Number:   5,
		Name:     "Rollback to previous image",
		Status:   report.Pass,
		Duration: time.Since(start),
	}
}

func check06Destroy(ctx context.Context, plug *docker.Plugin) report.Result {
	start := time.Now()

	err := plug.Destroy(ctx, testInstanceID)
	if err != nil {
		return report.Result{
			Number:   6,
			Name:     "Destroy instance",
			Status:   report.Fail,
			Duration: time.Since(start),
			Detail:   fmt.Sprintf("Destroy error: %v", err),
		}
	}

	time.Sleep(500 * time.Millisecond)

	if httpProbe(ctx, testPort) {
		return report.Result{
			Number:   6,
			Name:     "Destroy instance",
			Status:   report.Fail,
			Duration: time.Since(start),
			Detail:   "Instance still reachable after destroy",
		}
	}

	return report.Result{
		Number:   6,
		Name:     "Destroy instance",
		Status:   report.Pass,
		Duration: time.Since(start),
	}
}

func check07Idempotent(ctx context.Context, plug *docker.Plugin, imageRef plugins.ImageRef, dataDir string) report.Result {
	start := time.Now()
	id := "conf-plug-idem"

	provCtx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()

	err := plug.Provision(provCtx, plugins.ProvisionRequest{
		InstanceID: id,
		Image:      imageRef,
		Config:     plugins.InstanceConfig{AuthSecret: testAuthSecret},
		Port:       testPort + 1,
		DataDir:    dataDir,
	})
	if err != nil {
		return report.Result{
			Number:   7,
			Name:     "Idempotent provision (same ID twice)",
			Status:   report.Fail,
			Duration: time.Since(start),
			Detail:   fmt.Sprintf("First provision failed: %v", err),
		}
	}

	_ = plug.Destroy(ctx, id)

	provCtx2, cancel2 := context.WithTimeout(ctx, healthTimeout)
	defer cancel2()

	err = plug.Provision(provCtx2, plugins.ProvisionRequest{
		InstanceID: id,
		Image:      imageRef,
		Config:     plugins.InstanceConfig{AuthSecret: testAuthSecret},
		Port:       testPort + 1,
		DataDir:    dataDir,
	})
	if err != nil {
		return report.Result{
			Number:   7,
			Name:     "Idempotent provision (same ID twice)",
			Status:   report.Fail,
			Duration: time.Since(start),
			Detail:   fmt.Sprintf("Second provision failed: %v", err),
		}
	}

	return report.Result{
		Number:   7,
		Name:     "Idempotent provision (same ID twice)",
		Status:   report.Pass,
		Duration: time.Since(start),
	}
}

func check08NonexistentHealth(ctx context.Context, plug *docker.Plugin) report.Result {
	start := time.Now()

	_, err := plug.HealthCheck(ctx, "conf-nonexistent-instance-xyz")
	if err == nil {
		return report.Result{
			Number:   8,
			Name:     "HealthCheck nonexistent ID returns error",
			Status:   report.Fail,
			Duration: time.Since(start),
			Detail:   "HealthCheck did not return error for nonexistent ID",
		}
	}

	return report.Result{
		Number:   8,
		Name:     "HealthCheck nonexistent ID returns error",
		Status:   report.Pass,
		Duration: time.Since(start),
	}
}

func httpProbe(ctx context.Context, port int) bool {
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	c := &http.Client{Timeout: 3 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
