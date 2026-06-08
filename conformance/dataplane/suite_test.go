package dataplane_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/conformance/dataplane"
)

func TestDataPlaneConformance(t *testing.T) {
	base := os.Getenv("CONFORMANCE_DP_URL")
	secret := os.Getenv("CONFORMANCE_DP_SECRET")
	if base == "" || secret == "" {
		t.Skip("CONFORMANCE_DP_URL and CONFORMANCE_DP_SECRET not set; skipping data plane conformance suite")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	suite := &dataplane.Suite{BaseURL: base, AdminSecret: secret}
	result := suite.Run(ctx)

	t.Log("\n" + result.Format())

	if !result.OK() {
		t.Errorf("%d checks failed", result.Failed())
	}
}
