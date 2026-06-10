package plugin

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"

	"github.com/fox-in-the-box-ai/fox-fleet/conformance/runtime/report"
	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
	"github.com/fox-in-the-box-ai/fox-fleet/plugins/docker"
)

const (
	testInstanceID = "conf-plug-test"
	testPort       = 9177
	testAuthSecret = "conf-plug-secret-4e9a"
	healthTimeout  = 120 * time.Second
)

type Suite struct {
	Image string
}

func (s *Suite) Run(ctx context.Context) (*report.Suite, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("conformance: docker client: %w", err)
	}
	defer cli.Close()

	if isMovingTag(s.Image) {
		slog.Info("pulling image (moving tag)", "image", s.Image)
		rc, err := cli.ImagePull(ctx, s.Image, image.PullOptions{})
		if err != nil {
			return nil, fmt.Errorf("conformance: pull %s: %w", s.Image, err)
		}
		_, _ = io.Copy(io.Discard, rc)
		rc.Close()
	}

	plug := docker.NewWithClient(cli)
	defer plug.Close()

	result := &report.Suite{Image: s.Image}
	start := time.Now()

	imageRef := parseImageRef(s.Image)

	dataDir := filepath.Join(os.TempDir(), "fox-conformance-plugin", testInstanceID)
	_ = os.RemoveAll(dataDir)
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("conformance: create data dir: %w", err)
	}
	defer os.RemoveAll(filepath.Join(os.TempDir(), "fox-conformance-plugin"))

	s.runLifecycle(ctx, plug, result, imageRef, dataDir)
	s.runIdempotent(ctx, plug, result, imageRef)
	s.runErrorHandling(ctx, plug, result)

	result.Elapsed = time.Since(start)
	return result, nil
}

func (s *Suite) runLifecycle(ctx context.Context, plug *docker.Plugin, result *report.Suite, imageRef plugins.ImageRef, dataDir string) {
	result.Add(check01Provision(ctx, plug, imageRef, dataDir))
	defer func() {
		_ = plug.Destroy(context.WithoutCancel(ctx), testInstanceID)
	}()

	result.Add(check02HealthCheck(ctx, plug))
	result.Add(check03Configure(ctx, plug))
	result.Add(check04Rollout(ctx, plug, imageRef))
	result.Add(check05Rollback(ctx, plug, imageRef))
	result.Add(check06Destroy(ctx, plug))
}

func (s *Suite) runIdempotent(ctx context.Context, plug *docker.Plugin, result *report.Suite, imageRef plugins.ImageRef) {
	dataDir := filepath.Join(os.TempDir(), "fox-conformance-plugin", "idem-test")
	_ = os.MkdirAll(dataDir, 0o700)
	defer func() {
		_ = plug.Destroy(context.WithoutCancel(ctx), "conf-plug-idem")
		_ = os.RemoveAll(dataDir)
	}()

	result.Add(check07Idempotent(ctx, plug, imageRef, dataDir))
}

func (s *Suite) runErrorHandling(ctx context.Context, plug *docker.Plugin, result *report.Suite) {
	result.Add(check08NonexistentHealth(ctx, plug))
}

func isMovingTag(ref string) bool {
	if strings.Contains(ref, "@sha256:") {
		return false
	}
	i := strings.LastIndex(ref, ":")
	if i < 0 {
		return true
	}
	tag := ref[i+1:]
	return tag == "stable" || tag == "latest"
}

func parseImageRef(s string) plugins.ImageRef {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '@' {
			return plugins.ImageRef{
				Repository: s[:i],
				Digest:     s[i+1:],
			}
		}
	}
	return plugins.ImageRef{Repository: s}
}
