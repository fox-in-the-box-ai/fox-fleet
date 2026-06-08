package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/provisioner"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
)

const defaultPollInterval = 15 * time.Second

type Deps struct {
	Registry     *registry.Registry
	Provisioner  provisioner.Provisioner
	Plugin       plugins.DeploymentPlugin
	AdminSecret  string
	InstancePwd  string
	Image        plugins.ImageRef
	MaxInstances int
	PollInterval time.Duration
	Logger       *slog.Logger
}

type Server struct {
	registry    *registry.Registry
	provisioner provisioner.Provisioner
	plugin      plugins.DeploymentPlugin
	poller      *HealthPoller
	secret      []byte
	instPwd     string
	image       plugins.ImageRef
	maxInst     int
	log         *slog.Logger
	mux         *http.ServeMux
}

func NewServer(d Deps) *Server {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if d.PollInterval == 0 {
		d.PollInterval = defaultPollInterval
	}

	s := &Server{
		registry:    d.Registry,
		provisioner: d.Provisioner,
		plugin:      d.Plugin,
		secret:      []byte(d.AdminSecret),
		instPwd:     d.InstancePwd,
		image:       d.Image,
		maxInst:     d.MaxInstances,
		log:         d.Logger,
	}

	s.poller = &HealthPoller{
		registry: d.Registry,
		plugin:   d.Plugin,
		interval: d.PollInterval,
		cache:    make(map[string]plugins.HealthStatus),
		log:      d.Logger,
	}

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /api/instances", s.handleList)
	apiMux.HandleFunc("GET /api/instances/{id}", s.handleDetail)
	apiMux.HandleFunc("POST /api/instances", s.handleCreate)
	apiMux.HandleFunc("DELETE /api/instances/{id}", s.handleDestroy)

	s.mux = http.NewServeMux()
	s.mux.Handle("/api/", s.requireAuth(apiMux))

	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) Poller() *HealthPoller {
	return s.poller
}
