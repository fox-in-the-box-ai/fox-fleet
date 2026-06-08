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

// HealthPoller periodically checks instance health via the deployment plugin
// and caches results for the API handlers.
type HealthPoller struct {
	registry *registry.Registry
	plugin   plugins.DeploymentPlugin
	interval time.Duration
	mu       sync.RWMutex
	cache    map[string]plugins.HealthStatus
	log      *slog.Logger
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

	type result struct {
		id     string
		status plugins.HealthStatus
	}
	results := make([]result, len(instances))

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
			results[idx] = result{id: id, status: hs}
		}(i, inst.ID)
	}
	wg.Wait()

	newCache := make(map[string]plugins.HealthStatus, len(results))
	for _, r := range results {
		if r.id != "" {
			newCache[r.id] = r.status
		}
	}

	p.mu.Lock()
	p.cache = newCache
	p.mu.Unlock()
}

func (p *HealthPoller) Get(id string) (plugins.HealthStatus, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	hs, ok := p.cache[id]
	return hs, ok
}
