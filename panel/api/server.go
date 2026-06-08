package api

import (
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/source"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/events"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/provisioner"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/sessiontoken"
	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
)

const defaultPollInterval = 15 * time.Second

type Deps struct {
	Registry           *registry.Registry
	Provisioner        provisioner.Provisioner
	Plugin             plugins.DeploymentPlugin
	AdminSecret        string
	InstancePwd        string
	Image              plugins.ImageRef
	MaxInstances       int
	PollInterval       time.Duration
	Logger             *slog.Logger
	WebFS              fs.FS
	SourceRegistry     *source.Registry
	DataPlaneURL       string
	DefaultSkillset    string
	DefaultRole        string
	SkillsetsDir       string
	EventLog           *events.Log
	SigningKey         []byte
	SessionTokenTTL    time.Duration
	RateLimit          int
	ProvisionRateLimit int
	MetricsEnabled     bool
	QdrantHealth       QdrantHealthChecker
}

const defaultSessionTokenTTL = 10 * time.Minute

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
	signer          *sessiontoken.Signer
	sessionTTL      time.Duration
	inFlightMu      sync.Mutex
	inFlight        map[string]bool
	wg              sync.WaitGroup
	metrics         *metrics
}

func NewServer(d Deps) *Server {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if d.PollInterval == 0 {
		d.PollInterval = defaultPollInterval
	}

	if len(d.SigningKey) < 32 {
		panic("api: signing key must be at least 32 bytes")
	}

	ttl := d.SessionTokenTTL
	if ttl == 0 {
		ttl = defaultSessionTokenTTL
	}

	m := newMetrics()
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
		signer:          sessiontoken.NewSigner(d.SigningKey),
		sessionTTL:      ttl,
		inFlight:        make(map[string]bool),
		metrics:         m,
	}

	s.poller = &HealthPoller{
		registry: d.Registry,
		plugin:   d.Plugin,
		interval: d.PollInterval,
		cache:    make(map[string]plugins.HealthStatus),
		log:      d.Logger,
		qdrant:   d.QdrantHealth,
		events:   d.EventLog,
	}

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /api/instances", s.handleList)
	apiMux.HandleFunc("GET /api/instances/{id}", s.handleDetail)
	apiMux.HandleFunc("POST /api/instances", s.handleCreate)
	apiMux.HandleFunc("GET /api/instances/{id}/stats", s.handleInstanceStats)
	apiMux.HandleFunc("GET /api/instances/{id}/health-history", s.handleHealthHistory)
	apiMux.HandleFunc("DELETE /api/instances/{id}", s.handleDestroy)
	apiMux.HandleFunc("GET /api/sources", s.handleListSources)
	apiMux.HandleFunc("GET /api/sources/{id}", s.handleGetSource)
	apiMux.HandleFunc("GET /api/skillsets", s.handleListSkillsets)
	apiMux.HandleFunc("GET /api/skillsets/{name}", s.handleGetSkillset)
	apiMux.HandleFunc("POST /api/skillsets", s.handleUploadSkillset)
	apiMux.HandleFunc("GET /api/skillsets/{name}/download", s.handleDownloadSkillset)
	apiMux.HandleFunc("DELETE /api/skillsets/{name}", s.handleDeleteSkillset)
	apiMux.HandleFunc("POST /api/query", s.handleQuery)
	apiMux.HandleFunc("GET /api/events", s.handleEvents)
	apiMux.HandleFunc("POST /api/auth/session", s.handleSession)

	var apiHandler http.Handler = s.requireAuth(apiMux)

	rlRate := d.RateLimit
	if rlRate <= 0 {
		rlRate = 100
	}
	apiHandler = rateLimitMiddleware(newRateLimiter(rlRate))(apiHandler)
	apiHandler = metricsMiddleware(m)(apiHandler)

	s.mux = http.NewServeMux()
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	if d.MetricsEnabled {
		s.mux.Handle("GET /metrics", m.handler())
	}
	s.mux.Handle("/api/events/stream", s.requireSessionToken(http.HandlerFunc(s.handleEventsStream)))
	s.mux.Handle("/api/", apiHandler)

	if d.WebFS != nil {
		s.mux.Handle("/", securityHeaders(http.FileServerFS(d.WebFS)))
	}

	return s
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) Poller() *HealthPoller {
	return s.poller
}

func (s *Server) Wait() {
	s.wg.Wait()
}
