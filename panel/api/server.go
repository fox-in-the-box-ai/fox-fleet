package api

import (
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/source"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/events"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/provisioner"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
)

const defaultPollInterval = 15 * time.Second

type Deps struct {
	Registry        *registry.Registry
	Provisioner     provisioner.Provisioner
	Plugin          plugins.DeploymentPlugin
	AdminSecret     string
	InstancePwd     string
	Image           plugins.ImageRef
	MaxInstances    int
	PollInterval    time.Duration
	Logger          *slog.Logger
	WebFS           fs.FS
	SourceRegistry  *source.Registry
	DataPlaneURL    string
	DefaultSkillset string
	DefaultRole     string
	SkillsetsDir    string
	EventLog        *events.Log
}

type Server struct {
	registry        *registry.Registry
	provisioner     provisioner.Provisioner
	plugin          plugins.DeploymentPlugin
	poller          *HealthPoller
	secret          []byte
	instPwd         string
	image           plugins.ImageRef
	maxInst         int
	log             *slog.Logger
	mux             *http.ServeMux
	sources         *source.Registry
	dpURL           string
	defaultSkillset string
	defaultRole     string
	skillsetsDir    string
	events          *events.Log
}

func NewServer(d Deps) *Server {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if d.PollInterval == 0 {
		d.PollInterval = defaultPollInterval
	}

	s := &Server{
		registry:        d.Registry,
		provisioner:     d.Provisioner,
		plugin:          d.Plugin,
		secret:          []byte(d.AdminSecret),
		instPwd:         d.InstancePwd,
		image:           d.Image,
		maxInst:         d.MaxInstances,
		log:             d.Logger,
		sources:         d.SourceRegistry,
		dpURL:           d.DataPlaneURL,
		defaultSkillset: d.DefaultSkillset,
		defaultRole:     d.DefaultRole,
		skillsetsDir:    d.SkillsetsDir,
		events:          d.EventLog,
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
	apiMux.HandleFunc("GET /api/sources", s.handleListSources)
	apiMux.HandleFunc("GET /api/sources/{id}", s.handleGetSource)
	apiMux.HandleFunc("GET /api/skillsets", s.handleListSkillsets)
	apiMux.HandleFunc("GET /api/skillsets/{name}", s.handleGetSkillset)
	apiMux.HandleFunc("POST /api/skillsets", s.handleUploadSkillset)
	apiMux.HandleFunc("DELETE /api/skillsets/{name}", s.handleDeleteSkillset)
	apiMux.HandleFunc("POST /api/query", s.handleQuery)
	apiMux.HandleFunc("GET /api/events", s.handleEvents)

	s.mux = http.NewServeMux()
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.Handle("/api/", s.requireAuth(apiMux))

	if d.WebFS != nil {
		s.mux.Handle("/", http.FileServerFS(d.WebFS))
	}

	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) Poller() *HealthPoller {
	return s.poller
}
