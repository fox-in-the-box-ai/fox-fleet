package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/data-plane/source"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/config"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/provisioner"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
)

var validInstanceID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

type instanceItem struct {
	ID        string                `json:"id"`
	Status    string                `json:"status"`
	Port      int                   `json:"port"`
	CreatedAt string                `json:"created_at"`
	Health    *plugins.HealthStatus `json:"health,omitempty"`
}

type instanceDetail struct {
	ID            string                `json:"id"`
	Status        string                `json:"status"`
	Port          int                   `json:"port"`
	CreatedAt     string                `json:"created_at"`
	Health        *plugins.HealthStatus `json:"health,omitempty"`
	Logs          string                `json:"logs"`
	SkillsetName  string                `json:"skillset_name,omitempty"`
	PrincipalRole string                `json:"principal_role,omitempty"`
}

type createRequest struct {
	ID           string `json:"id"`
	SkillsetPath string `json:"skillset_path,omitempty"`
	Role         string `json:"role,omitempty"`
	Owner        string `json:"owner,omitempty"`
}

type createResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

const (
	maxLogBytes      = 1 << 20
	maxCreateBody    = 4096
	provisionTimeout = 5 * time.Minute
)

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	instances, err := s.registry.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "cannot list instances")
		return
	}

	items := make([]instanceItem, 0, len(instances))
	for _, inst := range instances {
		item := instanceItem{
			ID:        inst.ID,
			Status:    inst.Status,
			Port:      inst.Port,
			CreatedAt: inst.CreatedAt,
		}
		if hs, ok := s.poller.Get(inst.ID); ok {
			item.Health = &hs
		}
		items = append(items, item)
	}

	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validInstanceID.MatchString(id) {
		writeError(w, http.StatusBadRequest, "bad_request",
			fmt.Sprintf("invalid instance ID %q", id))
		return
	}

	inst, err := s.registry.Get(id)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found",
				fmt.Sprintf("instance %s not found", id))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "cannot retrieve instance details")
		return
	}

	resp := instanceDetail{
		ID:            inst.ID,
		Status:        inst.Status,
		Port:          inst.Port,
		CreatedAt:     inst.CreatedAt,
		SkillsetName:  inst.SkillsetName,
		PrincipalRole: inst.PrincipalRole,
	}

	if hs, ok := s.poller.Get(inst.ID); ok {
		resp.Health = &hs
	}

	rc, err := s.plugin.Logs(r.Context(), id, plugins.LogOpts{Tail: 100})
	if err == nil {
		data, _ := io.ReadAll(io.LimitReader(rc, maxLogBytes))
		rc.Close()
		resp.Logs = string(data)
	} else {
		s.log.Warn("failed to fetch logs", "instance", id, "error", err)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCreateBody)
	var body createRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "request body is not valid JSON")
		return
	}
	if body.ID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}
	if !validInstanceID.MatchString(body.ID) {
		writeError(w, http.StatusBadRequest, "bad_request",
			fmt.Sprintf("id must match [a-zA-Z0-9._-] and be 1-64 characters, got %q", body.ID))
		return
	}

	s.inFlightMu.Lock()
	if s.inFlight[body.ID] {
		s.inFlightMu.Unlock()
		writeError(w, http.StatusConflict, "conflict",
			fmt.Sprintf("instance %s is already being provisioned", body.ID))
		return
	}
	s.inFlight[body.ID] = true
	s.inFlightMu.Unlock()

	if _, err := s.registry.Get(body.ID); err == nil {
		s.clearInFlight(body.ID)
		writeError(w, http.StatusConflict, "conflict",
			fmt.Sprintf("instance %s already exists", body.ID))
		return
	}

	instances, err := s.registry.List()
	if err != nil {
		s.clearInFlight(body.ID)
		writeError(w, http.StatusInternalServerError, "internal_error", "cannot list instances")
		return
	}
	if len(instances) >= s.maxInst {
		s.clearInFlight(body.ID)
		writeError(w, http.StatusTooManyRequests, "cap_reached",
			fmt.Sprintf("maximum instance count reached (%d)", s.maxInst))
		return
	}

	skillset := body.SkillsetPath
	if skillset == "" {
		skillset = s.defaultSkillset
	}
	role := body.Role
	if role == "" {
		role = s.defaultRole
	}

	if s.events != nil {
		s.events.Emitf("provision", body.ID, "provisioning instance %s", body.ID)
	}
	s.metrics.incProvisions()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.clearInFlight(body.ID)
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), provisionTimeout)
		defer cancel()
		cloud := config.CloudConfig{
			Enabled: s.cloudCfg.Domain != "",
			Domain:  s.cloudCfg.Domain,
			Slug:    body.Owner,
		}
		_, err := s.provisioner.Provision(ctx, provisioner.Request{
			InstanceID:       body.ID,
			AdminSecret:      string(s.secret),
			InstancePassword: s.instPwd,
			Image:            s.image,
			DataPlaneURL:     s.dpURL,
			SkillsetPath:     skillset,
			PrincipalRole:    role,
			Cloud:            cloud,
		})
		if err != nil {
			s.log.Error("background provision failed",
				"instance", body.ID,
				"error", err)
			if s.events != nil {
				s.events.Emitf("error", body.ID, "provision failed: %v", err)
			}
		}
	}()

	writeJSON(w, http.StatusCreated, createResponse{
		ID:     body.ID,
		Status: "provisioning",
	})
}

func (s *Server) handleListSources(w http.ResponseWriter, _ *http.Request) {
	if s.sources == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	list, err := s.sources.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "cannot list sources")
		return
	}
	if list == nil {
		list = []source.Source{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGetSource(w http.ResponseWriter, r *http.Request) {
	if s.sources == nil {
		writeError(w, http.StatusNotFound, "not_found", "source registry is not configured — enable data_plane in config")
		return
	}
	id := r.PathValue("id")
	src, err := s.sources.Get(id)
	if err != nil {
		if errors.Is(err, source.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found",
				fmt.Sprintf("source %s not found", id))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "cannot retrieve source details")
		return
	}
	writeJSON(w, http.StatusOK, src)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if s.events == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, s.events.Recent(50))
}

func (s *Server) handleDestroy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validInstanceID.MatchString(id) {
		writeError(w, http.StatusBadRequest, "bad_request",
			fmt.Sprintf("invalid instance ID %q", id))
		return
	}
	removeData := r.URL.Query().Get("remove_data") == "true"

	if err := s.provisioner.Destroy(r.Context(), id, removeData); err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found",
				fmt.Sprintf("instance %s not found", id))
			return
		}
		s.log.Error("destroy failed", "instance", id, "error", err)
		writeError(w, http.StatusInternalServerError, "destroy_failed",
			fmt.Sprintf("cannot destroy instance %s — check server logs for details", id))
		return
	}

	if s.events != nil {
		s.events.Emitf("destroy", id, "destroyed instance %s", id)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleHealthHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validInstanceID.MatchString(id) {
		writeError(w, http.StatusBadRequest, "bad_request",
			fmt.Sprintf("invalid instance ID %q", id))
		return
	}

	if s.events == nil || s.events.Store() == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	since := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	evts, err := s.events.Store().ByInstance(id, since, 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "cannot retrieve health history")
		return
	}
	writeJSON(w, http.StatusOK, evts)
}

func (s *Server) handleInstanceStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validInstanceID.MatchString(id) {
		writeError(w, http.StatusBadRequest, "bad_request",
			fmt.Sprintf("invalid instance ID %q", id))
		return
	}

	stats, err := s.plugin.Stats(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "stats_unavailable",
			fmt.Sprintf("cannot retrieve stats for instance %s", id))
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) clearInFlight(id string) {
	s.inFlightMu.Lock()
	delete(s.inFlight, id)
	s.inFlightMu.Unlock()
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	setSecurityHeaders(w)
	status := "ok"
	resp := map[string]any{}
	if s.poller.qdrant != nil {
		if s.poller.QdrantHealthy() {
			resp["qdrant"] = "healthy"
		} else {
			resp["qdrant"] = "unhealthy"
			status = "degraded"
		}
	}
	resp["status"] = status
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
