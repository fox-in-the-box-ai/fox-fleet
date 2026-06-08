package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/embedding"
	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/ingestion"
	fileconn "github.com/fox-in-the-box-ai/fox-fleet/data-plane/ingestion/file"
	restconn "github.com/fox-in-the-box-ai/fox-fleet/data-plane/ingestion/rest"
	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/qdrant"
	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/source"
)

type Config struct {
	Listen          string
	AdminSecret     string
	Collection      string
	VectorSize      int
	AllowedFileDir  string
	DefaultMinScore float32
}

type QueryTokenValidator interface {
	ValidQueryToken(token string) bool
}

type Server struct {
	cfg            Config
	sources        *source.Registry
	embedder       *embedding.Client
	vector         *qdrant.Client
	fileConn       *fileconn.Connector
	restConn       *restconn.Connector
	log            *slog.Logger
	mux            *http.ServeMux
	tokenValidator QueryTokenValidator
}

func New(cfg Config, sources *source.Registry, embedder *embedding.Client, vector *qdrant.Client, log *slog.Logger, opts ...Option) *Server {
	if log == nil {
		log = slog.Default()
	}
	fc := fileconn.New(embedder, vector, cfg.AllowedFileDir)
	fc.SetTracker(sources)
	rc := restconn.New(embedder, vector)

	s := &Server{
		cfg:      cfg,
		sources:  sources,
		embedder: embedder,
		vector:   vector,
		fileConn: fc,
		restConn: rc,
		log:      log,
	}
	for _, o := range opts {
		o(s)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/health", s.handleHealth)
	mux.HandleFunc("GET /v1/readyz", s.handleReadyz)

	queryMux := http.NewServeMux()
	queryMux.HandleFunc("GET /v1/sources", s.handleListSources)
	queryMux.HandleFunc("POST /v1/query", s.handleQuery)
	mux.Handle("/v1/sources", s.requireQueryAuth(queryMux))
	mux.Handle("/v1/query", s.requireQueryAuth(queryMux))

	adminMux := http.NewServeMux()
	adminMux.HandleFunc("GET /v1/admin/sources", s.handleAdminListSources)
	adminMux.HandleFunc("POST /v1/admin/sources", s.handleAdminCreateSource)
	adminMux.HandleFunc("GET /v1/admin/sources/{id}", s.handleAdminGetSource)
	adminMux.HandleFunc("DELETE /v1/admin/sources/{id}", s.handleAdminDeleteSource)
	adminMux.HandleFunc("POST /v1/admin/sources/{id}/ingest", s.handleAdminIngestSource)

	mux.Handle("/v1/admin/", s.requireAdmin(adminMux))

	s.mux = mux
	return s
}

type Option func(*Server)

func WithTokenValidator(v QueryTokenValidator) Option {
	return func(s *Server) { s.tokenValidator = v }
}

func (s *Server) requireQueryAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if token == "" {
			token = r.Header.Get("X-Fox-Auth")
		}
		if token == "" {
			s.log.Warn("query auth missing credentials", "remote_addr", r.RemoteAddr, "path", r.URL.Path)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.AdminSecret)) == 1 {
			next.ServeHTTP(w, r)
			return
		}
		if s.tokenValidator != nil && s.tokenValidator.ValidQueryToken(token) {
			next.ServeHTTP(w, r)
			return
		}
		s.log.Warn("query auth invalid token", "remote_addr", r.RemoteAddr, "path", r.URL.Path)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	})
}

func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return h[len("Bearer "):]
	}
	return ""
}

func (s *Server) ValidateEmbeddingDimension(ctx context.Context) error {
	vecs, err := s.embedder.Embed(ctx, []string{"dimension validation probe"})
	if err != nil {
		s.log.Warn("embedding dimension validation skipped — service unreachable", "error", err)
		return nil
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		s.log.Warn("embedding dimension validation skipped — no vectors returned")
		return nil
	}
	actual := len(vecs[0])
	if actual != s.cfg.VectorSize {
		return fmt.Errorf("embedding dimension mismatch: config vector_size=%d but embedding model returned %d dimensions", s.cfg.VectorSize, actual)
	}
	s.log.Info("embedding dimension validated", "dimensions", actual)
	return nil
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) ListenAndServe() error {
	srv := &http.Server{
		Addr:              s.cfg.Listen,
		Handler:           s.mux,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      300 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	s.log.Info("data plane server listening", "addr", s.cfg.Listen)
	return srv.ListenAndServe()
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(s.cfg.AdminSecret) == 0 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		token := header[len("Bearer "):]
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.AdminSecret)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	healthy := s.vector.Healthy(r.Context())
	status := http.StatusOK
	state := "healthy"
	if !healthy {
		status = http.StatusServiceUnavailable
		state = "unhealthy"
	}
	writeJSON(w, status, map[string]string{"status": state})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if !s.vector.Healthy(r.Context()) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleListSources(w http.ResponseWriter, _ *http.Request) {
	list, err := s.sources.List()
	if err != nil {
		s.log.Error("list sources", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	if list == nil {
		list = []source.Source{}
	}
	writeJSON(w, http.StatusOK, list)
}

const defaultTopK = 5

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Query      string   `json:"query"`
		Collection string   `json:"collection"`
		TopK       int      `json:"top_k"`
		SourceID   string   `json:"source_id"`
		MinScore   *float32 `json:"min_score,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query is required"})
		return
	}
	if req.Collection == "" {
		req.Collection = s.cfg.Collection
	}
	if req.TopK <= 0 {
		req.TopK = defaultTopK
	}

	vectors, err := s.embedder.Embed(r.Context(), []string{req.Query})
	if err != nil {
		s.log.Error("query embed", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "embedding service unavailable"})
		return
	}
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "embedding service returned no vectors"})
		return
	}

	sr := qdrant.SearchRequest{
		Vector:      vectors[0],
		Limit:       req.TopK,
		WithPayload: true,
	}
	if req.SourceID != "" {
		sr.Filter = &qdrant.Filter{
			Must: []qdrant.Condition{{
				Key:   "source_id",
				Match: &qdrant.Match{Value: req.SourceID},
			}},
		}
	}

	results, err := s.vector.Search(r.Context(), req.Collection, sr)
	if err != nil {
		s.log.Error("query search", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "vector search unavailable"})
		return
	}

	minScore := s.cfg.DefaultMinScore
	if req.MinScore != nil {
		minScore = *req.MinScore
	}
	if minScore > 0 {
		filtered := results[:0]
		for _, r := range results {
			if r.Score >= minScore {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	type queryResult struct {
		Text     string  `json:"text"`
		Score    float32 `json:"score"`
		SourceID string  `json:"source_id,omitempty"`
		File     string  `json:"file,omitempty"`
		DocID    string  `json:"doc_id,omitempty"`
		ChunkIdx int     `json:"chunk_idx"`
	}

	out := make([]queryResult, 0, len(results))
	for _, r := range results {
		qr := queryResult{
			Score: r.Score,
		}
		if v, ok := r.Payload["text"].(string); ok {
			qr.Text = v
		}
		if v, ok := r.Payload["source_id"].(string); ok {
			qr.SourceID = v
		}
		if v, ok := r.Payload["file"].(string); ok {
			qr.File = v
		}
		if v, ok := r.Payload["doc_id"].(string); ok {
			qr.DocID = v
		}
		if v, ok := r.Payload["chunk_idx"].(float64); ok {
			qr.ChunkIdx = int(v)
		}
		out = append(out, qr)
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": out})
}

func (s *Server) handleAdminListSources(w http.ResponseWriter, r *http.Request) {
	s.handleListSources(w, r)
}

func (s *Server) handleAdminCreateSource(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		ID         string            `json:"id"`
		Type       string            `json:"type"`
		Name       string            `json:"name"`
		Collection string            `json:"collection"`
		Config     map[string]string `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.ID == "" || req.Type == "" || req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id, type, and name are required"})
		return
	}
	if req.Type != "file" && req.Type != "rest" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "type must be file or rest"})
		return
	}
	if req.Collection == "" {
		req.Collection = s.cfg.Collection
	}

	src := source.Source{
		ID:         req.ID,
		Type:       req.Type,
		Name:       req.Name,
		Collection: req.Collection,
		Config:     req.Config,
	}
	if err := s.sources.Create(src); err != nil {
		s.log.Error("create source", "error", err)
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": fmt.Sprintf("source %s already exists", req.ID)})
		} else {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		}
		return
	}

	got, err := s.sources.Get(req.ID)
	if err != nil {
		s.log.Error("get after create", "error", err)
		writeJSON(w, http.StatusCreated, src)
		return
	}
	writeJSON(w, http.StatusCreated, got)
}

func (s *Server) handleAdminGetSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	src, err := s.sources.Get(id)
	if errors.Is(err, source.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "source not found"})
		return
	}
	if err != nil {
		s.log.Error("get source", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	writeJSON(w, http.StatusOK, src)
}

func (s *Server) handleAdminDeleteSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	src, err := s.sources.Get(id)
	if errors.Is(err, source.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "source not found"})
		return
	}
	if err != nil {
		s.log.Error("get source for delete", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	col := src.Collection
	if col == "" {
		col = s.cfg.Collection
	}
	if err := s.vector.DeleteByFilter(r.Context(), col, qdrant.Filter{
		Must: []qdrant.Condition{{
			Key:   "source_id",
			Match: &qdrant.Match{Value: id},
		}},
	}); err != nil {
		s.log.Error("delete qdrant points for source", "source", id, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete vector points"})
		return
	}

	if err := s.sources.Delete(id); err != nil {
		s.log.Error("delete source", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	_ = s.sources.DeleteAllDocTracking(id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleAdminIngestSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	src, err := s.sources.Get(id)
	if errors.Is(err, source.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "source not found"})
		return
	}
	if err != nil {
		s.log.Error("get source for ingest", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	ingCfg := ingestionConfig(src, s.cfg.Collection)

	switch src.Type {
	case "file":
		if err := s.fileConn.Connect(r.Context(), ingCfg); err != nil {
			s.log.Error("file connect", "error", err)
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.logErr(s.sources.UpdateStatus(id, "ingesting"))
		result, err := s.fileConn.Ingest(r.Context(), id)
		if err != nil {
			s.logErr(s.sources.SetError(id, err.Error()))
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		s.logErr(s.sources.UpdateStatus(id, "active"))
		s.logErr(s.sources.UpdateCounts(id, result.DocumentsProcessed, result.ChunksStored))
		writeJSON(w, http.StatusOK, result)

	case "rest":
		if err := s.restConn.Connect(r.Context(), ingCfg); err != nil {
			s.log.Error("rest connect", "error", err)
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.logErr(s.sources.UpdateStatus(id, "ingesting"))
		result, err := s.restConn.Ingest(r.Context(), id)
		if err != nil {
			s.logErr(s.sources.SetError(id, err.Error()))
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		s.logErr(s.sources.UpdateStatus(id, "active"))
		s.logErr(s.sources.UpdateCounts(id, result.DocumentsProcessed, result.ChunksStored))
		writeJSON(w, http.StatusOK, result)

	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported source type"})
	}
}

func ingestionConfig(src source.Source, defaultCollection string) ingestion.SourceConfig {
	col := src.Collection
	if col == "" {
		col = defaultCollection
	}
	return ingestion.SourceConfig{
		SourceID:   src.ID,
		Type:       src.Type,
		Collection: col,
		Config:     src.Config,
	}
}

func (s *Server) logErr(err error) {
	if err != nil {
		s.log.Error("source registry update", "error", err)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
