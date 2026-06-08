package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
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
	_ "modernc.org/sqlite"

	plugconf "github.com/fox-in-the-box-ai/fox-fleet/conformance/plugin"
	conformance "github.com/fox-in-the-box-ai/fox-fleet/conformance/runtime"
	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/source"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/config"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/events"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/provisioner"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
	"github.com/fox-in-the-box-ai/fox-fleet/panel/api"
	"github.com/fox-in-the-box-ai/fox-fleet/panel/spa"
	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
	"github.com/fox-in-the-box-ai/fox-fleet/plugins/docker"
	"github.com/fox-in-the-box-ai/fox-fleet/rollout"
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
		newRolloutCmd(),
		newVersionCmd(),
		newConformanceCmd(),
		newVerifyCmd(),
		newSecCmd(),
		newGenerateSecretCmd(),
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

			webFS, err := fs.Sub(spa.Static, "static")
			if err != nil {
				return fmt.Errorf("cannot load embedded web assets: %w", err)
			}

			var srcReg *source.Registry
			var dpURL string
			if cfg.DataPlane.Enabled {
				dpURL = "http://" + cfg.DataPlane.Listen
				srcDB, err := sql.Open("sqlite", filepath.Join(cfg.Control.DataRoot, "sources.db"))
				if err != nil {
					return fmt.Errorf("cannot open sources database: %w", err)
				}
				srcDB.SetMaxOpenConns(1)
				defer srcDB.Close()
				srcReg, err = source.OpenRegistry(srcDB)
				if err != nil {
					return fmt.Errorf("cannot initialize source registry: %w", err)
				}
			}

			signingKey, err := reg.EnsureSigningKey()
			if err != nil {
				return fmt.Errorf("cannot initialize signing key: %w", err)
			}

			sessionTTL := time.Duration(cfg.Control.SessionTokenTTLSecs) * time.Second

			eventLog := events.NewLog(200)

			imageRef := parseImageRef(cfg.Docker.Image)
			if imageRef.Digest == "" {
				slog.Warn("docker.image uses a tag without a digest — pin to a digest for reproducible deployments (repo@sha256:...)",
					"image", cfg.Docker.Image)
			}

			apiServer := api.NewServer(api.Deps{
				Registry:        reg,
				Provisioner:     prov,
				Plugin:          plug,
				AdminSecret:     cfg.Auth.AdminSecret,
				InstancePwd:     cfg.Auth.InstancePassword,
				Image:           imageRef,
				MaxInstances:    cfg.Instances.MaxInstances,
				PollInterval:    pollInterval,
				WebFS:           webFS,
				SourceRegistry:  srcReg,
				DataPlaneURL:    dpURL,
				DefaultSkillset: cfg.Instances.DefaultSkillset,
				DefaultRole:     cfg.Instances.DefaultRole,
				SkillsetsDir:    filepath.Join(cfg.Control.DataRoot, "skillsets"),
				EventLog:        eventLog,
				SigningKey:      signingKey,
				SessionTokenTTL: sessionTTL,
			})

			ctx := cmd.Context()

			pollerCtx, pollerCancel := context.WithCancel(ctx)
			defer pollerCancel()
			go apiServer.Poller().Run(pollerCtx)

			srv := &http.Server{
				Addr:              cfg.Control.Listen,
				Handler:           apiServer.Handler(),
				ReadHeaderTimeout: 10 * time.Second,
				WriteTimeout:      30 * time.Second,
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

func newRolloutCmd() *cobra.Command {
	var image string

	cmd := &cobra.Command{
		Use:   "rollout",
		Short: "Rolling update of all instances to a new image",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ref := parseImageRef(image)
			if ref.Digest == "" {
				return fmt.Errorf("--image must be a digest reference (repo@sha256:...)")
			}

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

			orch := rollout.New(rollout.Options{
				Registry: reg,
				Plugin:   plug,
			})

			report, err := orch.Execute(cmd.Context(), ref)
			if err != nil {
				return err
			}

			fmt.Fprint(cmd.OutOrStdout(), report.Format())
			if !report.OK() {
				return fmt.Errorf("rollout completed with failures — see report above for details")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&image, "image", "", "target image digest (repo@sha256:...) (required)")
	_ = cmd.MarkFlagRequired("image")
	return cmd
}

func newConformanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conformance",
		Short: "Conformance testing",
	}
	cmd.AddCommand(newConformanceRunCmd())
	cmd.AddCommand(newConformancePluginCmd())
	return cmd
}

func newConformanceRunCmd() *cobra.Command {
	var image string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the runtime conformance suite against a Fox image",
		RunE: func(cmd *cobra.Command, _ []string) error {
			suite := &conformance.Suite{Image: image}
			result, err := suite.Run(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), result.Format())
			if !result.OK() {
				return fmt.Errorf("%d conformance checks failed", result.Failed())
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&image, "image", "", "Fox container image (required)")
	_ = cmd.MarkFlagRequired("image")
	return cmd
}

func newConformancePluginCmd() *cobra.Command {
	var image string

	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Run the plugin conformance suite against a Fox image",
		RunE: func(cmd *cobra.Command, _ []string) error {
			suite := &plugconf.Suite{Image: image}
			result, err := suite.Run(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), result.Format())
			if !result.OK() {
				return fmt.Errorf("%d plugin conformance checks failed", result.Failed())
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&image, "image", "", "Fox container image (required)")
	_ = cmd.MarkFlagRequired("image")
	return cmd
}

func newSecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sec",
		Short: "Security operations",
	}
	cmd.AddCommand(newSecRotateSSEKeyCmd())
	cmd.AddCommand(newSecRotateQueryTokenCmd())
	return cmd
}

func newSecRotateSSEKeyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rotate-sse-key",
		Short: "Rotate the SSE session token signing key",
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

			if _, err := reg.RotateSigningKey(); err != nil {
				return err
			}

			slog.Info("SSE signing key rotated — all active session tokens are now invalid")
			fmt.Fprintln(cmd.OutOrStdout(), "SSE signing key rotated. All active session tokens are now invalid.")
			fmt.Fprintln(cmd.OutOrStdout(), "Connected SSE clients will reconnect and obtain new tokens automatically.")
			return nil
		},
	}
}

func newSecRotateQueryTokenCmd() *cobra.Command {
	var instanceID string

	cmd := &cobra.Command{
		Use:   "rotate-query-token",
		Short: "Rotate the data plane query token for an instance",
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

			inst, err := reg.Get(instanceID)
			if err != nil {
				return fmt.Errorf("instance %s not found: %w", instanceID, err)
			}

			newToken, err := registry.GenerateQueryToken()
			if err != nil {
				return err
			}

			if err := reg.UpdateQueryToken(instanceID, newToken); err != nil {
				return err
			}

			injectParams := config.InjectParams{
				DataDir:          inst.DataDir,
				InstancePassword: cfg.Auth.InstancePassword,
				Config: plugins.InstanceConfig{
					AuthSecret:   cfg.Auth.AdminSecret,
					DataPlaneURL: "http://" + cfg.DataPlane.Listen,
				},
				QueryToken: newToken,
			}
			if err := config.Inject(injectParams); err != nil {
				return fmt.Errorf("re-inject config: %w", err)
			}

			prefix := newToken[:8]
			slog.Info("query token rotated", "instance", instanceID, "token_prefix", prefix)
			fmt.Fprintf(cmd.OutOrStdout(), "Query token rotated for instance %s (prefix: %s).\n", instanceID, prefix)
			fmt.Fprintln(cmd.OutOrStdout(), "Restart the instance for the new token to take effect.")
			return nil
		},
	}
	cmd.Flags().StringVar(&instanceID, "instance", "", "instance ID (required)")
	_ = cmd.MarkFlagRequired("instance")
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
		return nil, nil, fmt.Errorf("cannot connect to Docker at %s: %w", cfg.Docker.Socket, err)
	}

	return reg, docker.NewWithClient(cli), nil
}

func newGenerateSecretCmd() *cobra.Command {
	var length int
	cmd := &cobra.Command{
		Use:   "generate-secret",
		Short: "Generate a cryptographically random secret suitable for admin_secret",
		RunE: func(_ *cobra.Command, _ []string) error {
			buf := make([]byte, length)
			if _, err := rand.Read(buf); err != nil {
				return fmt.Errorf("generate secret: %w", err)
			}
			fmt.Println(hex.EncodeToString(buf))
			return nil
		},
	}
	cmd.Flags().IntVar(&length, "bytes", 32, "number of random bytes (output is hex-encoded, so 32 bytes = 64 chars)")
	return cmd
}
