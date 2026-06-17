package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"unicode/utf8"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/cloud"
)

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type updateUserRequest struct {
	Password   *string `json:"password,omitempty"`
	InstanceID *string `json:"instance_id,omitempty"`
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if req.Username == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "username is required")
		return
	}
	if utf8.RuneCountInString(req.Username) > 64 {
		writeError(w, http.StatusBadRequest, "bad_request", "username must be 1-64 characters")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "bad_request", "password must be at least 8 characters")
		return
	}

	u, err := s.users.Create(req.Username, req.Password)
	if errors.Is(err, cloud.ErrUserExists) {
		writeError(w, http.StatusConflict, "conflict", "user already exists")
		return
	}
	if err != nil {
		s.log.Error("create user failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "cannot create user")
		return
	}

	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) handleListUsers(w http.ResponseWriter, _ *http.Request) {
	users, err := s.users.List()
	if err != nil {
		s.log.Error("list users failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "cannot list users")
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	u, err := s.users.Get(username)
	if errors.Is(err, cloud.ErrUserNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	if err != nil {
		s.log.Error("get user failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "cannot get user")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if req.Password != nil && len(*req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "bad_request", "password must be at least 8 characters")
		return
	}

	u, err := s.users.Update(username, req.Password, req.InstanceID)
	if errors.Is(err, cloud.ErrUserNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	if err != nil {
		s.log.Error("update user failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "cannot update user")
		return
	}

	writeJSON(w, http.StatusOK, u)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	if err := s.users.Delete(username); errors.Is(err, cloud.ErrUserNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "user not found")
		return
	} else if err != nil {
		s.log.Error("delete user failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "cannot delete user")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
