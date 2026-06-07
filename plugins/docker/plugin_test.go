package docker

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"

	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
)

func TestInterfaceCompliance(t *testing.T) {
	var _ plugins.DeploymentPlugin = (*Plugin)(nil)
}

func TestExtractHostPort(t *testing.T) {
	tests := []struct {
		name string
		info types.ContainerJSON
		want int
	}{
		{
			name: "valid port binding",
			info: types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					HostConfig: &container.HostConfig{
						PortBindings: nat.PortMap{
							nat.Port(internalPort): []nat.PortBinding{
								{HostIP: "127.0.0.1", HostPort: "9001"},
							},
						},
					},
				},
			},
			want: 9001,
		},
		{
			name: "no bindings",
			info: types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					HostConfig: &container.HostConfig{
						PortBindings: nat.PortMap{},
					},
				},
			},
			want: 0,
		},
		{
			name: "empty binding list",
			info: types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					HostConfig: &container.HostConfig{
						PortBindings: nat.PortMap{
							nat.Port(internalPort): []nat.PortBinding{},
						},
					},
				},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractHostPort(tt.info)
			if got != tt.want {
				t.Errorf("extractHostPort() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestContainerNaming(t *testing.T) {
	want := "fox-test-instance"
	got := containerPrefix + "test-instance"
	if got != want {
		t.Errorf("container name = %q, want %q", got, want)
	}
}

func TestHttpProbeUnreachable(t *testing.T) {
	if httpProbe(context.Background(), 1, "/health") {
		t.Error("httpProbe should return false for unreachable port")
	}
}
