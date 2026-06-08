package sut

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type Mode int

const (
	Standalone    Mode = iota
	Managed                    // FOX_PLANE_AUTH_SECRET + HERMES_WEBUI_PASSWORD
	BootInvariant              // FOX_PLANE_AUTH_SECRET only (expects exit)
)

type Config struct {
	Image      string
	Mode       Mode
	AuthSecret string
	Password   string
	ExtraEnv   map[string]string
}

type Handle struct {
	ContainerID string
	Name        string
	Port        int
	BaseURL     string
	AuthSecret  string
	Password    string
	dataDir     string
	cli         *client.Client
}

const stopTimeoutSec = 10

func (h *Handle) Cleanup(ctx context.Context) {
	if h == nil {
		return
	}
	timeout := stopTimeoutSec
	_ = h.cli.ContainerStop(ctx, h.ContainerID, container.StopOptions{Timeout: &timeout})
	_ = h.cli.ContainerRemove(ctx, h.ContainerID, container.RemoveOptions{Force: true})
	if h.dataDir != "" {
		_ = os.RemoveAll(h.dataDir)
	}
}

func (h *Handle) WaitExit(ctx context.Context, timeout time.Duration) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	waitC, errC := h.cli.ContainerWait(ctx, h.ContainerID, container.WaitConditionNotRunning)
	select {
	case result := <-waitC:
		return int(result.StatusCode), nil
	case err := <-errC:
		return -1, fmt.Errorf("sut: wait exit %s: %w", h.Name, err)
	case <-ctx.Done():
		return -1, fmt.Errorf("sut: container did not exit within %s", timeout)
	}
}
