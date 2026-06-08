package runtime_test

import (
	"context"
	"os"
	"testing"
	"time"

	runtime "github.com/fox-in-the-box-ai/fox-fleet/conformance/runtime"
)

func TestRuntimeConformance(t *testing.T) {
	image := os.Getenv("CONFORMANCE_IMAGE")
	if image == "" {
		t.Skip("CONFORMANCE_IMAGE not set; skipping runtime conformance suite")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	suite := &runtime.Suite{Image: image}
	result, err := suite.Run(ctx)
	if err != nil {
		t.Fatalf("suite failed to run: %v", err)
	}

	t.Log("\n" + result.Format())

	if !result.OK() {
		t.Errorf("%d checks failed", result.Failed())
	}
}
