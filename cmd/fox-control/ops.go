package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/output"
)

func newBackupCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Create a consistent backup of all databases",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := LoadConfig(cfgPath)
			if err != nil {
				return err
			}
			if outputPath == "" {
				return fmt.Errorf("--output is required")
			}

			if err := os.MkdirAll(outputPath, 0o755); err != nil {
				return fmt.Errorf("create backup directory: %w", err)
			}

			dbs := []string{"registry.db"}
			sourcesPath := filepath.Join(cfg.Control.DataRoot, "sources.db")
			if _, err := os.Stat(sourcesPath); err == nil {
				dbs = append(dbs, "sources.db")
			}
			eventsPath := filepath.Join(cfg.Control.DataRoot, "events.db")
			if _, err := os.Stat(eventsPath); err == nil {
				dbs = append(dbs, "events.db")
			}

			for _, name := range dbs {
				src := filepath.Join(cfg.Control.DataRoot, name)
				dst := filepath.Join(outputPath, name)
				if err := vacuumInto(src, dst); err != nil {
					return fmt.Errorf("backup %s: %w", name, err)
				}
				slog.Info("backed up", "db", name, "to", dst)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Backup complete: %d databases to %s\n", len(dbs), outputPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&outputPath, "output", "", "directory to write backup files (required)")
	return cmd
}

func vacuumInto(src, dst string) error {
	db, err := sql.Open("sqlite", src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`VACUUM INTO ?`, dst)
	return err
}

func newRestoreCmd() *cobra.Command {
	var inputPath string

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore databases from a backup",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := LoadConfig(cfgPath)
			if err != nil {
				return err
			}
			if inputPath == "" {
				return fmt.Errorf("--input is required")
			}

			for _, name := range []string{"registry.db", "sources.db", "events.db"} {
				src := filepath.Join(inputPath, name)
				if _, err := os.Stat(src); os.IsNotExist(err) {
					continue
				}
				dst := filepath.Join(cfg.Control.DataRoot, name)

				if isLocked(dst) {
					return fmt.Errorf("%s is locked by a running server — stop fox-control before restoring", name)
				}

				data, err := os.ReadFile(src)
				if err != nil {
					return fmt.Errorf("read backup %s: %w", name, err)
				}
				if err := os.WriteFile(dst, data, 0o644); err != nil {
					return fmt.Errorf("write %s: %w", name, err)
				}
				slog.Info("restored", "db", name, "from", src)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Restore complete. Restart fox-control to use the restored databases.")
			return nil
		},
	}
	cmd.Flags().StringVar(&inputPath, "input", "", "directory containing backup files (required)")
	return cmd
}

func isLocked(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return false
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`BEGIN EXCLUSIVE; ROLLBACK`)
	return err != nil
}

type diagCheck struct {
	Name   string
	Status string
	Detail string
}

func newDiagnosticsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diagnostics",
		Short: "Run health checks and report status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := LoadConfig(cfgPath)
			if err != nil {
				return err
			}

			var checks []diagCheck

			checks = append(checks, diagCheck{Name: "config", Status: "ok", Detail: "valid"})

			checks = append(checks, checkDocker(cfg))
			checks = append(checks, checkRegistry(cfg))
			checks = append(checks, checkDisk(cfg))
			checks = append(checks, checkPort(cfg))

			if cfg.Qdrant.Enabled {
				checks = append(checks, checkQdrant(cfg))
			}
			if cfg.DataPlane.Enabled && cfg.Embedding.BaseURL != "" {
				checks = append(checks, checkEmbedding(cfg))
			}
			if cfg.DataPlane.Enabled {
				checks = append(checks, checkDataPlanePort(cfg))
			}

			ofmt, ferr := output.ParseFormat(outputFormat)
			if ferr != nil {
				return ferr
			}
			w := output.NewWriter(cmd.OutOrStdout(), ofmt)
			headers := []string{"CHECK", "STATUS", "DETAIL"}
			rows := make([][]string, len(checks))
			anyFail := false
			for i, c := range checks {
				rows[i] = []string{c.Name, c.Status, c.Detail}
				if c.Status == "fail" {
					anyFail = true
				}
			}
			if err := w.WriteTable(headers, rows); err != nil {
				return err
			}
			if anyFail {
				return fmt.Errorf("one or more diagnostics failed")
			}
			return nil
		},
	}
}

func checkDocker(cfg *Config) diagCheck {
	host := "unix://" + cfg.Docker.Socket
	cli, err := client.NewClientWithOpts(
		client.WithHost(host),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return diagCheck{Name: "docker", Status: "fail", Detail: err.Error()}
	}
	defer cli.Close()
	_, err = cli.Ping(nil)
	if err != nil {
		return diagCheck{Name: "docker", Status: "fail", Detail: err.Error()}
	}
	return diagCheck{Name: "docker", Status: "ok", Detail: cfg.Docker.Socket}
}

func checkRegistry(cfg *Config) diagCheck {
	dbPath := filepath.Join(cfg.Control.DataRoot, "registry.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return diagCheck{Name: "registry", Status: "fail", Detail: err.Error()}
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var result string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil {
		return diagCheck{Name: "registry", Status: "fail", Detail: err.Error()}
	}
	if result != "ok" {
		return diagCheck{Name: "registry", Status: "fail", Detail: result}
	}
	return diagCheck{Name: "registry", Status: "ok", Detail: "integrity_check ok"}
}

func checkDisk(cfg *Config) diagCheck {
	info, err := os.Stat(cfg.Control.DataRoot)
	if err != nil {
		return diagCheck{Name: "disk", Status: "fail", Detail: err.Error()}
	}
	if !info.IsDir() {
		return diagCheck{Name: "disk", Status: "fail", Detail: "data_root is not a directory"}
	}
	return diagCheck{Name: "disk", Status: "ok", Detail: cfg.Control.DataRoot}
}

func checkPort(cfg *Config) diagCheck {
	ln, err := net.DialTimeout("tcp", cfg.Control.Listen, 2*time.Second)
	if err != nil {
		return diagCheck{Name: "port", Status: "ok", Detail: cfg.Control.Listen + " available"}
	}
	ln.Close()
	return diagCheck{Name: "port", Status: "warn", Detail: cfg.Control.Listen + " in use (server running?)"}
}

func checkQdrant(cfg *Config) diagCheck {
	addr := fmt.Sprintf("localhost:%d", cfg.Qdrant.HTTPPort)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return diagCheck{Name: "qdrant", Status: "fail", Detail: addr + " unreachable"}
	}
	conn.Close()
	return diagCheck{Name: "qdrant", Status: "ok", Detail: addr + " reachable"}
}

func checkEmbedding(cfg *Config) diagCheck {
	return diagCheck{Name: "embedding", Status: "ok", Detail: cfg.Embedding.BaseURL + " configured"}
}

func checkDataPlanePort(cfg *Config) diagCheck {
	ln, err := net.DialTimeout("tcp", cfg.DataPlane.Listen, 2*time.Second)
	if err != nil {
		return diagCheck{Name: "data_plane_port", Status: "ok", Detail: cfg.DataPlane.Listen + " available"}
	}
	ln.Close()
	return diagCheck{Name: "data_plane_port", Status: "warn", Detail: cfg.DataPlane.Listen + " in use (data plane running?)"}
}
