package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/docker/docker/client"
	"github.com/spf13/cobra"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/provisioner"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
	"github.com/fox-in-the-box-ai/fox-fleet/panel/api"
	"github.com/fox-in-the-box-ai/fox-fleet/plugins/docker"
)

var (
	buildVersion = "dev"
	buildCommit  = "unknown"
	buildDate    = "unknown"
)

var cfgPath string

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	root := newRootCmd()
	if err := root.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "fox-control",
		Short:        "Fox Fleet management plane",
		SilenceUsage: true,
	}
	cmd.PersistentFlags().StringVar(&cfgPath, "config", "/etc/fox-control/fox-control.toml", "path to config file")
	cmd.AddCommand(
		newServeCmd(),
		newProvisionCmd(),
		newDestroyCmd(),
		newListCmd(),
		newVersionCmd(),
	)
	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "fox-control %s (commit %s, built %s)\n",
				buildVersion, buildCommit, buildDate)
		},
	}
}

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the panel HTTP server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := LoadConfig(cfgPath)
			if err != nil {
				return err
			}

			reg, plug, err := openRegistryAndPlugin(cfg)
			if err != nil {
				return err
			}
			defer reg.Close()
			defer plug.Close()

			prov := provisioner.New(provisioner.Options{
				Registry:       reg,
				Plugin:         plug,
				DataRoot:       cfg.Control.DataRoot,
				PortRangeStart: cfg.Instances.PortStart,
				MaxInstances:   cfg.Instances.MaxInstances,
			})

			pollInterval := time.Duration(cfg.Control.HealthPollSeconds) * time.Second

			apiServer := api.NewServer(api.Deps{
				Registry:     reg,
				Provisioner:  prov,
				Plugin:       plug,
				AdminSecret:  cfg.Auth.AdminSecret,
				InstancePwd:  cfg.Auth.InstancePassword,
				Image:        parseImageRef(cfg.Docker.Image),
				MaxInstances: cfg.Instances.MaxInstances,
				PollInterval: pollInterval,
			})

			ctx := cmd.Context()

			pollerCtx, pollerCancel := context.WithCancel(ctx)
			defer pollerCancel()
			go apiServer.Poller().Run(pollerCtx)

			srv := &http.Server{
				Addr:              cfg.Control.Listen,
				Handler:           apiServer.Handler(),
				ReadHeaderTimeout: 10 * time.Second,
			}

			go func() {
				<-ctx.Done()
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := srv.Shutdown(shutdownCtx); err != nil {
						slog.Error("server shutdown error", "error", err)
					}
			}()

			slog.Info("panel server listening", "addr", cfg.Control.Listen)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	}
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List provisioned instances",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := LoadConfig(cfgPath)
			if err != nil {
				return err
			}

			reg, err := registry.Open(filepath.Join(cfg.Control.DataRoot, "registry.db"))
			if err != nil {
				return err
			}
			defer reg.Close()

			instances, err := reg.List()
			if err != nil {
				return err
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tPORT\tSTATUS\tIMAGE\tCREATED")
			for _, inst := range instances {
				fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\n",
					inst.ID, inst.Port, inst.Status, inst.ImageDigest, inst.CreatedAt)
			}
			return w.Flush()
		},
	}
}

func newProvisionCmd() *cobra.Command {
	var instanceID string

	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Provision a new Fox instance",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := LoadConfig(cfgPath)
			if err != nil {
				return err
			}

			reg, plug, err := openRegistryAndPlugin(cfg)
			if err != nil {
				return err
			}
			defer reg.Close()
			defer plug.Close()

			prov := provisioner.New(provisioner.Options{
				Registry:       reg,
				Plugin:         plug,
				DataRoot:       cfg.Control.DataRoot,
				PortRangeStart: cfg.Instances.PortStart,
				MaxInstances:   cfg.Instances.MaxInstances,
			})

			inst, err := prov.Provision(cmd.Context(), provisioner.Request{
				InstanceID:       instanceID,
				AdminSecret:      cfg.Auth.AdminSecret,
				InstancePassword: cfg.Auth.InstancePassword,
				Image:            parseImageRef(cfg.Docker.Image),
			})
			if err != nil {
				return err
			}

			slog.Info("instance provisioned",
				"id", inst.ID,
				"port", inst.Port,
				"data_dir", inst.DataDir,
			)
			return nil
		},
	}
	cmd.Flags().StringVar(&instanceID, "id", "", "instance ID (required)")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newDestroyCmd() *cobra.Command {
	var (
		instanceID string
		removeData bool
	)

	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy a Fox instance",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := LoadConfig(cfgPath)
			if err != nil {
				return err
			}

			reg, plug, err := openRegistryAndPlugin(cfg)
			if err != nil {
				return err
			}
			defer reg.Close()
			defer plug.Close()

			prov := provisioner.New(provisioner.Options{
				Registry:       reg,
				Plugin:         plug,
				DataRoot:       cfg.Control.DataRoot,
				PortRangeStart: cfg.Instances.PortStart,
				MaxInstances:   cfg.Instances.MaxInstances,
			})

			if err := prov.Destroy(cmd.Context(), instanceID, removeData); err != nil {
				return err
			}

			slog.Info("instance destroyed",
				"id", instanceID,
				"data_removed", removeData,
			)
			return nil
		},
	}
	cmd.Flags().StringVar(&instanceID, "id", "", "instance ID (required)")
	cmd.Flags().BoolVar(&removeData, "remove-data", false, "also remove instance data directory")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func openRegistryAndPlugin(cfg *Config) (*registry.Registry, *docker.Plugin, error) {
	reg, err := registry.Open(filepath.Join(cfg.Control.DataRoot, "registry.db"))
	if err != nil {
		return nil, nil, err
	}

	host := "unix://" + cfg.Docker.Socket
	if runtime.GOOS == "windows" {
		host = "npipe://" + cfg.Docker.Socket
	}

	cli, err := client.NewClientWithOpts(
		client.WithHost(host),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		reg.Close()
		return nil, nil, fmt.Errorf("docker client: %w", err)
	}

	return reg, docker.NewWithClient(cli), nil
}
