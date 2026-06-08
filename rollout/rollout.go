package rollout

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
)

const (
	defaultHealthTimeout = 120 * time.Second
)

type ResultStatus string

const (
	StatusRolledOut  ResultStatus = "rolled_out"
	StatusRolledBack ResultStatus = "rolled_back"
	StatusSkipped    ResultStatus = "skipped"
)

type InstanceResult struct {
	ID       string           `json:"id"`
	Status   ResultStatus     `json:"status"`
	Previous plugins.ImageRef `json:"previous"`
	Error    string           `json:"error,omitempty"`
}

type Report struct {
	Target    plugins.ImageRef `json:"target"`
	Instances []InstanceResult `json:"instances"`
}

func (r *Report) OK() bool {
	for _, inst := range r.Instances {
		if inst.Status != StatusRolledOut {
			return false
		}
	}
	return true
}

func (r *Report) Format() string {
	var s string
	s += fmt.Sprintf("Rollout target: %s@%s\n\n", r.Target.Repository, r.Target.Digest)
	for _, inst := range r.Instances {
		line := fmt.Sprintf("  %-20s %s", inst.ID, inst.Status)
		if inst.Error != "" {
			line += fmt.Sprintf("  (%s)", inst.Error)
		}
		s += line + "\n"
	}
	return s
}

type Options struct {
	Registry       *registry.Registry
	Plugin         plugins.DeploymentPlugin
	HealthTimeout  time.Duration
	HealthInterval func(elapsed time.Duration) time.Duration
	Logger         *slog.Logger
}

func DefaultHealthInterval(elapsed time.Duration) time.Duration {
	if elapsed < 30*time.Second {
		return 2 * time.Second
	}
	return 5 * time.Second
}

type Orchestrator struct {
	registry       *registry.Registry
	plugin         plugins.DeploymentPlugin
	healthTimeout  time.Duration
	healthInterval func(elapsed time.Duration) time.Duration
	log            *slog.Logger
}

func New(opts Options) *Orchestrator {
	if opts.HealthTimeout == 0 {
		opts.HealthTimeout = defaultHealthTimeout
	}
	if opts.HealthInterval == nil {
		opts.HealthInterval = DefaultHealthInterval
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Orchestrator{
		registry:       opts.Registry,
		plugin:         opts.Plugin,
		healthTimeout:  opts.HealthTimeout,
		healthInterval: opts.HealthInterval,
		log:            opts.Logger,
	}
}

func (o *Orchestrator) Execute(ctx context.Context, target plugins.ImageRef) (*Report, error) {
	instances, err := o.registry.List()
	if err != nil {
		return nil, fmt.Errorf("rollout: list instances: %w", err)
	}

	if len(instances) == 0 {
		return &Report{Target: target}, nil
	}

	report := &Report{
		Target:    target,
		Instances: make([]InstanceResult, 0, len(instances)),
	}

	for _, inst := range instances {
		if err := ctx.Err(); err != nil {
			for _, remaining := range instances[len(report.Instances)+1:] {
				report.Instances = append(report.Instances, InstanceResult{
					ID:     remaining.ID,
					Status: StatusSkipped,
					Previous: plugins.ImageRef{
						Repository: target.Repository,
						Digest:     remaining.ImageDigest,
					},
					Error: "aborted due to prior failure",
				})
			}
			break
		}

		result := o.rolloutInstance(ctx, inst, target)
		report.Instances = append(report.Instances, result)

		if result.Status != StatusRolledOut {
			for _, remaining := range instances[len(report.Instances):] {
				report.Instances = append(report.Instances, InstanceResult{
					ID:     remaining.ID,
					Status: StatusSkipped,
					Previous: plugins.ImageRef{
						Repository: target.Repository,
						Digest:     remaining.ImageDigest,
					},
					Error: "aborted due to prior failure",
				})
			}
			break
		}
	}

	return report, nil
}

func (o *Orchestrator) rolloutInstance(ctx context.Context, inst registry.Instance, target plugins.ImageRef) InstanceResult {
	previous := plugins.ImageRef{
		Repository: target.Repository,
		Digest:     inst.ImageDigest,
	}

	o.log.Info("rolling out instance", "id", inst.ID, "from", inst.ImageDigest, "to", target.Digest)

	if err := o.plugin.Rollout(ctx, inst.ID, target); err != nil {
		o.log.Error("rollout failed", "id", inst.ID, "error", err)
		return InstanceResult{
			ID:       inst.ID,
			Status:   StatusRolledBack,
			Previous: previous,
			Error:    fmt.Sprintf("rollout: %v", err),
		}
	}

	if err := o.waitHealthy(ctx, inst.ID); err != nil {
		o.log.Error("health check failed after rollout, rolling back", "id", inst.ID, "error", err)

		rbCtx := context.WithoutCancel(ctx)
		if rbErr := o.plugin.Rollback(rbCtx, inst.ID, previous); rbErr != nil {
			o.log.Error("rollback also failed", "id", inst.ID, "error", rbErr)
			return InstanceResult{
				ID:       inst.ID,
				Status:   StatusRolledBack,
				Previous: previous,
				Error:    fmt.Sprintf("health failed: %v; rollback failed: %v", err, rbErr),
			}
		}

		return InstanceResult{
			ID:       inst.ID,
			Status:   StatusRolledBack,
			Previous: previous,
			Error:    fmt.Sprintf("health failed: %v", err),
		}
	}

	if err := o.registry.UpdateImageDigest(inst.ID, target.Digest); err != nil {
		o.log.Error("failed to update registry after successful rollout", "id", inst.ID, "error", err)
	}

	o.log.Info("instance rolled out successfully", "id", inst.ID)
	return InstanceResult{
		ID:       inst.ID,
		Status:   StatusRolledOut,
		Previous: previous,
	}
}

func (o *Orchestrator) waitHealthy(ctx context.Context, instanceID string) error {
	deadline := time.Now().Add(o.healthTimeout)
	healthCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	start := time.Now()
	for {
		status, err := o.plugin.HealthCheck(healthCtx, instanceID)
		if err == nil && status.Healthy {
			return nil
		}

		elapsed := time.Since(start)
		if time.Now().After(deadline) {
			return fmt.Errorf("health check timed out after %s", elapsed.Round(time.Second))
		}

		wait := o.healthInterval(elapsed)
		select {
		case <-healthCtx.Done():
			return healthCtx.Err()
		case <-time.After(wait):
		}
	}
}
