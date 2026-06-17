package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/cloud"
)

type cloudContextKey struct{}

type CloudUser struct {
	Username string
}

type CloudConfig struct {
	CookieName     string
	Secure         bool
	Domain         string
	SessionTTL     time.Duration
	LoginRateLimit int
}

func (c CloudConfig) loginRate() int {
	if c.LoginRateLimit > 0 {
		return c.LoginRateLimit
	}
	return 10
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleCloudLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCreateBody)
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "username and password are required")
		return
	}

	_, err := s.users.Authenticate(req.Username, req.Password)
	if errors.Is(err, cloud.ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
		return
	}
	if errors.Is(err, cloud.ErrUserNotFound) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
		return
	}
	if err != nil {
		s.log.Error("cloud login failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "login failed")
		return
	}

	token, _, err := s.sessions.Create(req.Username, s.cloudCfg.SessionTTL)
	if err != nil {
		s.log.Error("create session failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "login failed")
		return
	}

	http.SetCookie(w, s.sessionCookie(token, s.cloudCfg.SessionTTL))
	writeJSON(w, http.StatusOK, map[string]string{"username": req.Username})
}

func (s *Server) handleCloudLogout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(s.cloudCfg.CookieName)
	if err != nil || c.Value == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not logged in")
		return
	}

	_ = s.sessions.Delete(c.Value)
	http.SetCookie(w, s.sessionCookie("", -time.Hour))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) requireCloudSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(s.cloudCfg.CookieName)
		if err != nil || c.Value == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "session required")
			return
		}

		sess, err := s.sessions.Validate(c.Value)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "session expired or invalid")
			return
		}

		ctx := context.WithValue(r.Context(), cloudContextKey{}, CloudUser{Username: sess.UserID})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func CloudUserFromContext(ctx context.Context) (CloudUser, bool) {
	u, ok := ctx.Value(cloudContextKey{}).(CloudUser)
	return u, ok
}

func (s *Server) sessionCookie(token string, maxAge time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:     s.cloudCfg.CookieName,
		Value:    token,
		Path:     "/",
		Domain:   s.cloudCfg.Domain,
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		Secure:   s.cloudCfg.Secure,
		SameSite: http.SameSiteLaxMode,
	}
}
