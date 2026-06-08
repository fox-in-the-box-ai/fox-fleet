package api

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
)

const maxConcurrentChecks = 4

type QdrantHealthChecker interface {
	Healthy(ctx context.Context) bool
}

type AutoRestartConfig struct {
	Enabled   bool
	Threshold int
	Cooldown  time.Duration
}

type HealthPoller struct {
	registry      *registry.Registry
	plugin        plugins.DeploymentPlugin
	interval      time.Duration
	mu            sync.RWMutex
	cache         map[string]plugins.HealthStatus
	log           *slog.Logger
	qdrant        QdrantHealthChecker
	qdrantHealthy bool
	autoRestart   AutoRestartConfig
	unhealthyRuns map[string]int
	lastRestart   map[string]time.Time
}

func (p *HealthPoller) Run(ctx context.Context) {
	p.poll(ctx)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *HealthPoller) poll(ctx context.Context) {
	instances, err := p.registry.List()
	if err != nil {
		p.log.Error("health poll: list instances", "error", err)
		return
	}

	results := make([]pollResult, len(instances))

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentChecks)

loop:
	for i, inst := range instances {
		select {
		case <-ctx.Done():
			break loop
		case sem <- struct{}{}:
		}
		wg.Add(1)
		go func(idx int, id string) {
			defer func() { <-sem; wg.Done() }()
			hs, err := p.plugin.HealthCheck(ctx, id)
			if err != nil {
				p.log.Warn("health check failed", "instance", id, "error", err)
				hs = plugins.HealthStatus{CheckedAt: time.Now().UTC()}
			}
			results[idx] = pollResult{id: id, status: hs}
		}(i, inst.ID)
	}
	wg.Wait()

	newCache := make(map[string]plugins.HealthStatus, len(results))
	for _, r := range results {
		if r.id != "" {
			newCache[r.id] = r.status
		}
	}

	qHealth := true
	if p.qdrant != nil {
		qHealth = p.qdrant.Healthy(ctx)
	}

	p.mu.Lock()
	if p.qdrant != nil && p.qdrantHealthy != qHealth {
		if qHealth {
			p.log.Info("qdrant health restored")
		} else {
			p.log.Warn("qdrant is unhealthy")
		}
	}
	p.cache = newCache
	p.qdrantHealthy = qHealth
	p.mu.Unlock()

	if p.autoRestart.Enabled {
		p.checkAutoRestart(ctx, results)
	}
}

type pollResult struct {
	id     string
	status plugins.HealthStatus
}

func (p *HealthPoller) checkAutoRestart(ctx context.Context, results []pollResult) {
	if p.unhealthyRuns == nil {
		p.unhealthyRuns = make(map[string]int)
		p.lastRestart = make(map[string]time.Time)
	}

	for _, r := range results {
		if r.id == "" {
			continue
		}
		if r.status.Healthy {
			delete(p.unhealthyRuns, r.id)
			continue
		}
		p.unhealthyRuns[r.id]++
		if p.unhealthyRuns[r.id] < p.autoRestart.Threshold {
			continue
		}
		if last, ok := p.lastRestart[r.id]; ok && time.Since(last) < p.autoRestart.Cooldown {
			continue
		}
		p.log.Warn("auto-restarting unhealthy instance",
			"instance", r.id,
			"consecutive_failures", p.unhealthyRuns[r.id])
		if err := p.plugin.Restart(ctx, r.id); err != nil {
			p.log.Error("auto-restart failed", "instance", r.id, "error", err)
		} else {
			p.lastRestart[r.id] = time.Now()
			p.unhealthyRuns[r.id] = 0
		}
	}
}

func (p *HealthPoller) Get(id string) (plugins.HealthStatus, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	hs, ok := p.cache[id]
	return hs, ok
}

func (p *HealthPoller) QdrantHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.qdrantHealthy
}
