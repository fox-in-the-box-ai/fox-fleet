package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/cloud"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/config"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/provisioner"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
)

var reservedSlugs = map[string]bool{
	"admin":   true,
	"api":     true,
	"www":     true,
	"mail":    true,
	"login":   true,
	"logout":  true,
	"cloud":   true,
	"fleet":   true,
	"fox":     true,
	"static":  true,
	"assets":  true,
	"healthz": true,
	"robots":  true,
}

var validSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,61}[a-z0-9]$`)

func isValidSlug(s string) bool {
	if len(s) == 0 || len(s) > 63 {
		return false
	}
	if len(s) == 1 {
		return s[0] >= 'a' && s[0] <= 'z' || s[0] >= '0' && s[0] <= '9'
	}
	return validSlug.MatchString(s) && !strings.Contains(s, "--")
}

type provisionRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Slug     string `json:"slug"`
}

type provisionResponse struct {
	InstanceID string `json:"instance_id"`
	Username   string `json:"username"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
}

func (s *Server) handleProvision(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCreateBody)
	var req provisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if req.Username == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "username is required")
		return
	}
	if !validCloudUsername.MatchString(req.Username) {
		writeError(w, http.StatusBadRequest, "bad_request",
			"username must be 1-63 lowercase alphanumeric or hyphen characters, must start and end with alphanumeric")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "bad_request", "password must be at least 8 characters")
		return
	}
	if len(req.Password) > 72 {
		writeError(w, http.StatusBadRequest, "bad_request", "password too long")
		return
	}
	if req.Slug == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "slug is required")
		return
	}
	if !isValidSlug(req.Slug) {
		writeError(w, http.StatusBadRequest, "bad_request",
			"slug must be a DNS label: 1-63 lowercase alphanumeric or hyphen characters, no leading/trailing/consecutive hyphens")
		return
	}
	if reservedSlugs[req.Slug] {
		writeError(w, http.StatusConflict, "slug_reserved",
			fmt.Sprintf("slug %q is reserved", req.Slug))
		return
	}

	// Use slug as the instance ID for subdomain routing.
	instanceID := req.Slug

	// Check in-flight guard.
	s.inFlightMu.Lock()
	if s.inFlight[instanceID] {
		s.inFlightMu.Unlock()
		writeError(w, http.StatusConflict, "conflict",
			fmt.Sprintf("instance %s is already being provisioned", instanceID))
		return
	}
	s.inFlight[instanceID] = true
	s.inFlightMu.Unlock()

	// Check instance doesn't already exist.
	if _, err := s.registry.Get(instanceID); err == nil {
		s.clearInFlight(instanceID)
		writeError(w, http.StatusConflict, "slug_taken",
			fmt.Sprintf("slug %q is already in use", req.Slug))
		return
	}

	// Check capacity.
	instances, err := s.registry.List()
	if err != nil {
		s.clearInFlight(instanceID)
		writeError(w, http.StatusInternalServerError, "internal_error", "cannot list instances")
		return
	}
	if len(instances) >= s.maxInst {
		s.clearInFlight(instanceID)
		writeError(w, http.StatusTooManyRequests, "cap_reached",
			fmt.Sprintf("maximum instance count reached (%d)", s.maxInst))
		return
	}

	// Create user (instance binding happens after provisioner registers the instance).
	user, err := s.users.Create(req.Username, req.Password)
	if errors.Is(err, cloud.ErrUserExists) {
		s.clearInFlight(instanceID)
		writeError(w, http.StatusConflict, "user_exists", "username already taken")
		return
	}
	if err != nil {
		s.clearInFlight(instanceID)
		s.log.Error("provision: create user failed", "username", req.Username, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "cannot create user")
		return
	}

	if s.events != nil {
		s.events.Emitf("provision", instanceID, "provisioning instance %s for user %s", instanceID, req.Username)
	}
	s.metrics.incProvisions()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.clearInFlight(instanceID)
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), provisionTimeout)
		defer cancel()
		cc := config.CloudConfig{
			Enabled: s.cloudCfg.Domain != "",
			Domain:  s.cloudCfg.Domain,
			Slug:    req.Slug,
		}
		_, provErr := s.provisioner.Provision(ctx, provisioner.Request{
			InstanceID:       instanceID,
			AdminSecret:      string(s.secret),
			InstancePassword: s.instPwd,
			Image:            s.image,
			DataPlaneURL:     s.dpURL,
			SkillsetPath:     s.defaultSkillset,
			PrincipalRole:    s.defaultRole,
			Cloud:            cc,
		})
		if provErr != nil {
			s.log.Error("provision: background provision failed",
				"instance", instanceID,
				"username", req.Username,
				"error", provErr)
			if s.events != nil {
				s.events.Emitf("error", instanceID, "provision failed: %v", provErr)
			}
			s.rollbackUser(req.Username)
			return
		}
		if err := s.users.SetInstanceID(req.Username, instanceID); err != nil {
			s.log.Error("provision: bind user to instance failed",
				"username", req.Username,
				"instance", instanceID,
				"error", err)
			if s.events != nil {
				s.events.Emitf("error", instanceID, "bind user %s failed: %v", req.Username, err)
			}
		}
	}()

	writeJSON(w, http.StatusCreated, provisionResponse{
		InstanceID: instanceID,
		Username:   req.Username,
		Status:     "provisioning",
		CreatedAt:  user.CreatedAt,
	})
}

func (s *Server) rollbackUser(username string) {
	if err := s.users.Delete(username); err != nil {
		s.log.Error("provision: rollback user delete failed", "username", username, "error", err)
	}
}

type slugCheckResponse struct {
	Slug      string `json:"slug"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

func (s *Server) handleCheckSlug(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	if !isValidSlug(slug) {
		writeJSON(w, http.StatusOK, slugCheckResponse{
			Slug:   slug,
			Reason: "invalid DNS label",
		})
		return
	}

	if reservedSlugs[slug] {
		writeJSON(w, http.StatusOK, slugCheckResponse{
			Slug:   slug,
			Reason: "reserved",
		})
		return
	}

	// Check in-flight.
	s.inFlightMu.Lock()
	inFlight := s.inFlight[slug]
	s.inFlightMu.Unlock()
	if inFlight {
		writeJSON(w, http.StatusOK, slugCheckResponse{
			Slug:   slug,
			Reason: "currently being provisioned",
		})
		return
	}

	// Check registry.
	if _, err := s.registry.Get(slug); err == nil {
		writeJSON(w, http.StatusOK, slugCheckResponse{
			Slug:   slug,
			Reason: "already in use",
		})
		return
	} else if !errors.Is(err, registry.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "internal_error", "cannot check slug availability")
		return
	}

	writeJSON(w, http.StatusOK, slugCheckResponse{
		Slug:      slug,
		Available: true,
	})
}
