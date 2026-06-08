package dataplane

import (
	"context"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/conformance/runtime/report"
)

type Suite struct {
	BaseURL     string
	AdminSecret string
}

func (s *Suite) Run(ctx context.Context) *report.Suite {
	result := &report.Suite{Image: s.BaseURL}
	start := time.Now()

	result.Add(check01Health(ctx, s.BaseURL, s.AdminSecret))
	result.Add(check02Readyz(ctx, s.BaseURL, s.AdminSecret))
	result.Add(check03AdminAuthRequired(ctx, s.BaseURL))
	result.Add(check04AdminInvalidAuth(ctx, s.BaseURL))
	result.Add(check05AdminListSources(ctx, s.BaseURL, s.AdminSecret))
	result.Add(check06SourceCRUD(ctx, s.BaseURL, s.AdminSecret))
	result.Add(check07QueryAuthRequired(ctx, s.BaseURL))
	result.Add(check08QueryWithAuth(ctx, s.BaseURL, s.AdminSecret))
	result.Add(check09SourceListConsistency(ctx, s.BaseURL, s.AdminSecret))
	result.Add(check10HealthContentType(ctx, s.BaseURL))

	result.Elapsed = time.Since(start)
	return result
}
