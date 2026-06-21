package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
)

type upgradeRequest struct {
	TargetImage string `json:"target_image"`
}

type upgradeResponse struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	PreviousDigest string `json:"previous_digest,omitempty"`
	CurrentDigest  string `json:"current_digest"`
}

func (s *Server) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validInstanceID.MatchString(id) {
		writeError(w, http.StatusBadRequest, "bad_request",
			fmt.Sprintf("invalid instance ID %q", id))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCreateBody)
	var req upgradeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	if req.TargetImage == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "target_image is required")
		return
	}

	target, err := parseImageRef(req.TargetImage)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	s.inFlightMu.Lock()
	if s.inFlight[id] {
		s.inFlightMu.Unlock()
		writeError(w, http.StatusConflict, "conflict",
			fmt.Sprintf("instance %s has an operation in progress", id))
		return
	}
	s.inFlight[id] = true
	s.inFlightMu.Unlock()
	defer s.clearInFlight(id)

	inst, err := s.registry.Get(id)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found",
				fmt.Sprintf("instance %s not found", id))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "cannot retrieve instance")
		return
	}

	if target.Digest != "" && inst.ImageDigest == target.Digest {
		writeJSON(w, http.StatusOK, upgradeResponse{
			ID:            id,
			Status:        "already_current",
			CurrentDigest: inst.ImageDigest,
		})
		return
	}

	previousDigest := inst.ImageDigest

	if s.events != nil {
		s.events.Emitf("upgrade_start", id, "upgrading instance %s to %s", id, req.TargetImage)
	}

	ctx, cancel := context.WithTimeout(r.Context(), provisionTimeout)
	defer cancel()

	if err := s.plugin.Rollout(ctx, id, target); err != nil {
		s.log.Error("upgrade rollout failed", "instance", id, "error", err)
		if s.events != nil {
			s.events.Emitf("upgrade_failed", id, "rollout failed: %v", err)
		}
		rbRef := plugins.ImageRef{Repository: target.Repository, Digest: previousDigest}
		if rbErr := s.plugin.Rollback(ctx, id, rbRef); rbErr != nil {
			s.log.Error("upgrade rollback also failed", "instance", id, "error", rbErr)
		}
		writeError(w, http.StatusInternalServerError, "upgrade_failed",
			fmt.Sprintf("rollout failed for instance %s", id))
		return
	}

	newDigest := target.Digest
	if newDigest == "" {
		newDigest = "unknown"
	}
	if err := s.registry.UpdateImageDigest(id, newDigest); err != nil {
		s.log.Error("upgrade registry update failed", "instance", id, "error", err)
	}

	if s.events != nil {
		s.events.Emitf("upgrade_complete", id, "upgraded instance %s", id)
	}

	s.log.Info("instance upgraded", "instance", id, "previous", previousDigest, "current", newDigest)

	writeJSON(w, http.StatusOK, upgradeResponse{
		ID:             id,
		Status:         "upgraded",
		PreviousDigest: previousDigest,
		CurrentDigest:  newDigest,
	})
}

func parseImageRef(raw string) (plugins.ImageRef, error) {
	if !strings.Contains(raw, "/") {
		return plugins.ImageRef{}, fmt.Errorf("invalid image reference %q: must contain a registry path", raw)
	}
	if strings.Contains(raw, "://") {
		return plugins.ImageRef{}, fmt.Errorf("invalid image reference %q: must not contain a scheme", raw)
	}

	if idx := strings.Index(raw, "@sha256:"); idx >= 0 {
		return plugins.ImageRef{
			Repository: raw[:idx],
			Digest:     raw[idx+1:],
		}, nil
	}

	if idx := strings.LastIndex(raw, ":"); idx > strings.LastIndex(raw, "/") {
		return plugins.ImageRef{
			Repository: raw,
			Digest:     "",
		}, nil
	}

	return plugins.ImageRef{
		Repository: raw,
		Digest:     "",
	}, nil
}
