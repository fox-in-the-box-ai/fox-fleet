package qdrant

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestConfigDefaults(t *testing.T) {
	var cfg Config
	cfg.applyDefaults()

	if cfg.Image != defaultImage {
		t.Errorf("Image = %q, want %q", cfg.Image, defaultImage)
	}
	if cfg.HTTPPort != 6333 {
		t.Errorf("HTTPPort = %d, want 6333", cfg.HTTPPort)
	}
	if cfg.GRPCPort != 6334 {
		t.Errorf("GRPCPort = %d, want 6334", cfg.GRPCPort)
	}
}

func TestConfigPreservesExplicit(t *testing.T) {
	cfg := Config{
		Image:    "qdrant/qdrant:v1.12.0",
		HTTPPort: 7333,
		GRPCPort: 7334,
		DataDir:  "/tmp/qdrant",
	}
	cfg.applyDefaults()

	if cfg.Image != "qdrant/qdrant:v1.12.0" {
		t.Errorf("Image = %q, want %q", cfg.Image, "qdrant/qdrant:v1.12.0")
	}
	if cfg.HTTPPort != 7333 {
		t.Errorf("HTTPPort = %d, want 7333", cfg.HTTPPort)
	}
	if cfg.GRPCPort != 7334 {
		t.Errorf("GRPCPort = %d, want 7334", cfg.GRPCPort)
	}
}

func TestConfigValidateMissingDataDir(t *testing.T) {
	cfg := Config{Image: "qdrant/qdrant:v1.14.1", HTTPPort: 6333, GRPCPort: 6334}
	if err := cfg.validate(); err != ErrMissingDataDir {
		t.Errorf("validate() = %v, want ErrMissingDataDir", err)
	}
}

func TestConfigValidateValid(t *testing.T) {
	cfg := Config{DataDir: "/tmp/qdrant"}
	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		t.Errorf("validate() = %v", err)
	}
}

func TestNewManagerMissingDataDir(t *testing.T) {
	_, err := NewManager(nil, Config{})
	if err != ErrMissingDataDir {
		t.Errorf("NewManager() = %v, want ErrMissingDataDir", err)
	}
}

func TestNewManagerValid(t *testing.T) {
	m, err := NewManager(nil, Config{DataDir: "/tmp/qdrant"})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if m.cfg.Image != defaultImage {
		t.Errorf("Image = %q, want %q", m.cfg.Image, defaultImage)
	}
}

func TestURLs(t *testing.T) {
	m := &Manager{cfg: Config{HTTPPort: 6333, GRPCPort: 6334}}
	if got := m.HTTPURL(); got != "http://127.0.0.1:6333" {
		t.Errorf("HTTPURL() = %q, want %q", got, "http://127.0.0.1:6333")
	}
	if got := m.GRPCURL(); got != "127.0.0.1:6334" {
		t.Errorf("GRPCURL() = %q, want %q", got, "127.0.0.1:6334")
	}
}

func TestHttpProbeHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, portStr, _ := net.SplitHostPort(srv.Listener.Addr().String())
	port, _ := strconv.Atoi(portStr)

	if !httpProbe(context.Background(), port, "/healthz") {
		t.Error("httpProbe should return true for healthy server")
	}
}

func TestHttpProbeUnhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, portStr, _ := net.SplitHostPort(srv.Listener.Addr().String())
	port, _ := strconv.Atoi(portStr)

	if httpProbe(context.Background(), port, "/healthz") {
		t.Error("httpProbe should return false for unhealthy server")
	}
}

func TestHttpProbeUnreachable(t *testing.T) {
	if httpProbe(context.Background(), 1, "/healthz") {
		t.Error("httpProbe should return false for unreachable port")
	}
}

func TestPollHealthTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, portStr, _ := net.SplitHostPort(srv.Listener.Addr().String())
	port, _ := strconv.Atoi(portStr)

	m := &Manager{cfg: Config{HTTPPort: port}}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := m.pollHealth(ctx)
	if err == nil {
		t.Fatal("pollHealth should return error on timeout")
	}
}
