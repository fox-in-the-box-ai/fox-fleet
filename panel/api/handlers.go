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
	ID        string                `json:"id"`
	Status    string                `json:"status"`
	Port      int                   `json:"port"`
	CreatedAt string                `json:"created_at"`
	Health    *plugins.HealthStatus `json:"health,omitempty"`
	Logs      string                `json:"logs"`
}

type createRequest struct {
	ID string `json:"id"`
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
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to list instances")
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

	inst, err := s.registry.Get(id)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found",
				fmt.Sprintf("instance %s not found", id))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to get instance")
		return
	}

	resp := instanceDetail{
		ID:        inst.ID,
		Status:    inst.Status,
		Port:      inst.Port,
		CreatedAt: inst.CreatedAt,
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
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	if body.ID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "id is required")
		return
	}
	if !validInstanceID.MatchString(body.ID) {
		writeError(w, http.StatusBadRequest, "bad_request", "id must be alphanumeric (max 64 chars)")
		return
	}

	instances, err := s.registry.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to list instances")
		return
	}
	for _, inst := range instances {
		if inst.ID == body.ID {
			writeError(w, http.StatusConflict, "conflict",
				fmt.Sprintf("instance %s already exists", body.ID))
			return
		}
	}
	if len(instances) >= s.maxInst {
		writeError(w, http.StatusTooManyRequests, "cap_reached",
			fmt.Sprintf("maximum instance count reached (%d)", s.maxInst))
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), provisionTimeout)
		defer cancel()
		_, err := s.provisioner.Provision(ctx, provisioner.Request{
			InstanceID:       body.ID,
			AdminSecret:      string(s.secret),
			InstancePassword: s.instPwd,
			Image:            s.image,
			DataPlaneURL:     s.dpURL,
		})
		if err != nil {
			s.log.Error("background provision failed",
				"instance", body.ID,
				"error", err)
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
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to list sources")
		return
	}
	if list == nil {
		list = []source.Source{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleDestroy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	removeData := r.URL.Query().Get("remove_data") == "true"

	if err := s.provisioner.Destroy(r.Context(), id, removeData); err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found",
				fmt.Sprintf("instance %s not found", id))
			return
		}
		s.log.Error("destroy failed", "instance", id, "error", err)
		writeError(w, http.StatusInternalServerError, "destroy_failed", "failed to destroy instance")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
