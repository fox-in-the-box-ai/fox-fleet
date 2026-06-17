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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"

	plugconf "github.com/fox-in-the-box-ai/fox-fleet/conformance/plugin"
	conformance "github.com/fox-in-the-box-ai/fox-fleet/conformance/runtime"
	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/embedding"
	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/qdrant"
	dpserver "github.com/fox-in-the-box-ai/fox-fleet/data-plane/server"
	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/source"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/cloud"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/config"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/events"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/output"
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

var (
	cfgPath      string
	outputFormat string
)

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
	cmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "output format: table, json, quiet")
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
		newBackupCmd(),
		newRestoreCmd(),
		newDiagnosticsCmd(),
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

			configureLogging(cfg)

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
				if cfg.TLS.CertFile != "" {
					dpURL = "https://" + cfg.DataPlane.Listen
				} else {
					dpURL = "http://" + cfg.DataPlane.Listen
				}
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

			var vectorClient *qdrant.Client
			var qdrantHealth api.QdrantHealthChecker
			if cfg.Qdrant.Enabled {
				qdrantURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Qdrant.HTTPPort)
				vectorClient = qdrant.NewClient(qdrantURL)
				qdrantHealth = vectorClient
			}

			var dpSrv *http.Server
			if cfg.DataPlane.Enabled {
				embedClient := embedding.NewClient(embedding.Config{
					BaseURL: cfg.Embedding.BaseURL,
					APIKey:  cfg.Embedding.APIKey,
					Model:   cfg.Embedding.Model,
				})

				dpS := dpserver.New(dpserver.Config{
					Listen:      cfg.DataPlane.Listen,
					AdminSecret: cfg.Auth.AdminSecret,
					Collection:  cfg.DataPlane.Collection,
					VectorSize:  cfg.DataPlane.VectorSize,
				}, srcReg, embedClient, vectorClient, nil, dpserver.WithTokenValidator(reg))

				dpSrv = &http.Server{
					Addr:              cfg.DataPlane.Listen,
					Handler:           dpS.Handler(),
					ReadHeaderTimeout: 10 * time.Second,
					WriteTimeout:      300 * time.Second,
				}
			}

			signingKey, err := reg.EnsureSigningKey()
			if err != nil {
				return fmt.Errorf("cannot initialize signing key: %w", err)
			}

			sessionTTL := time.Duration(cfg.Control.SessionTokenTTLSecs) * time.Second

			eventsDB, err := sql.Open("sqlite", filepath.Join(cfg.Control.DataRoot, "events.db"))
			if err != nil {
				return fmt.Errorf("cannot open events database: %w", err)
			}
			eventsDB.SetMaxOpenConns(1)
			defer eventsDB.Close()
			eventStore, err := events.OpenStore(eventsDB)
			if err != nil {
				return fmt.Errorf("cannot initialize event store: %w", err)
			}
			eventLog := events.NewPersistentLog(200, eventStore)

			if len(cfg.Webhooks) > 0 {
				targets := make([]events.WebhookTarget, len(cfg.Webhooks))
				for i, wh := range cfg.Webhooks {
					targets[i] = events.WebhookTarget{
						URL:    wh.URL,
						Events: wh.Events,
						Secret: wh.Secret,
					}
				}
				eventLog.SetWebhooks(events.NewWebhookDispatcher(targets))
			}

			imageRef := parseImageRef(cfg.Docker.Image)
			if imageRef.Digest == "" {
				slog.Warn("docker.image uses a tag without a digest — pin to a digest for reproducible deployments (repo@sha256:...)",
					"image", cfg.Docker.Image)
			}

			metricsEnabled := cfg.Control.MetricsEnabled == nil || *cfg.Control.MetricsEnabled

			var userStore *cloud.UserStore
			var sessionStore *cloud.SessionStore
			var cloudCfg api.CloudConfig
			if cfg.Cloud.Enabled {
				db := reg.DB()
				userStore = cloud.NewUserStore(db)
				sessionStore = cloud.NewSessionStore(db)

				cloudTTL := 24 * time.Hour
				if cfg.Cloud.SessionTTL > 0 {
					cloudTTL = time.Duration(cfg.Cloud.SessionTTL) * time.Second
				}
				cookieName := cfg.Cloud.CookieName
				if cookieName == "" {
					cookieName = "fox_session"
				}
				cloudCfg = api.CloudConfig{
					CookieName:     cookieName,
					Secure:         cfg.TLS.CertFile != "",
					Domain:         cfg.Cloud.Domain,
					SessionTTL:     cloudTTL,
					LoginRateLimit: cfg.Cloud.LoginRateLimit,
				}

				purgeCtx, purgeCancel := context.WithCancel(cmd.Context())
				defer purgeCancel()
				sessionStore.StartPurge(purgeCtx, 10*time.Minute, slog.Default())
			}

			apiServer := api.NewServer(api.Deps{
				Registry:           reg,
				Provisioner:        prov,
				Plugin:             plug,
				AdminSecret:        cfg.Auth.AdminSecret,
				InstancePwd:        cfg.Auth.InstancePassword,
				Image:              imageRef,
				MaxInstances:       cfg.Instances.MaxInstances,
				PollInterval:       pollInterval,
				WebFS:              webFS,
				SourceRegistry:     srcReg,
				DataPlaneURL:       dpURL,
				DefaultSkillset:    cfg.Instances.DefaultSkillset,
				DefaultRole:        cfg.Instances.DefaultRole,
				SkillsetsDir:       filepath.Join(cfg.Control.DataRoot, "skillsets"),
				EventLog:           eventLog,
				SigningKey:         signingKey,
				SessionTokenTTL:    sessionTTL,
				MetricsEnabled:     metricsEnabled,
				QdrantHealth:       qdrantHealth,
				RateLimit:          cfg.RateLimit.RequestsPerMinute,
				ProvisionRateLimit: cfg.RateLimit.ProvisionPerMinute,
				AutoRestart: api.AutoRestartConfig{
					Enabled:   cfg.AutoRestart.Enabled,
					Threshold: cfg.AutoRestart.Threshold,
					Cooldown:  time.Duration(cfg.AutoRestart.CooldownSeconds) * time.Second,
				},
				UserStore:    userStore,
				SessionStore: sessionStore,
				Cloud:        cloudCfg,
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

			if dpSrv != nil {
				go func() {
					slog.Info("data plane server listening", "addr", dpSrv.Addr)
					if err := dpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
						slog.Error("data plane server error", "error", err)
					}
				}()
			}

			go func() {
				<-ctx.Done()
				slog.Info("shutting down: stopping new requests")
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if dpSrv != nil {
					if err := dpSrv.Shutdown(shutdownCtx); err != nil {
						slog.Error("data plane shutdown error", "error", err)
					}
				}
				if err := srv.Shutdown(shutdownCtx); err != nil {
					slog.Error("server shutdown error", "error", err)
				}
			}()

			if cfg.TLS.CertFile != "" {
				slog.Info("panel server listening (TLS)", "addr", cfg.Control.Listen)
				if err := srv.ListenAndServeTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
					return err
				}
			} else {
				slog.Info("panel server listening", "addr", cfg.Control.Listen)
				if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					return err
				}
			}
			slog.Info("waiting for in-flight provisions to complete")
			apiServer.Wait()
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

			ofmt, ferr := output.ParseFormat(outputFormat)
			if ferr != nil {
				return ferr
			}
			w := output.NewWriter(cmd.OutOrStdout(), ofmt)
			headers := []string{"ID", "PORT", "STATUS", "IMAGE", "CREATED"}
			rows := make([][]string, len(instances))
			for i, inst := range instances {
				rows[i] = []string{
					inst.ID,
					fmtInt(inst.Port),
					inst.Status,
					inst.ImageDigest,
					inst.CreatedAt,
				}
			}
			return w.WriteTable(headers, rows)
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

func fmtInt(n int) string { return strconv.Itoa(n) }

func configureLogging(cfg *Config) {
	var level slog.Level
	switch strings.ToLower(cfg.Control.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if cfg.Control.LogFormat == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(handler))
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
