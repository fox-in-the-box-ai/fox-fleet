package api

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/sessiontoken"
)

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if subtle.ConstantTimeCompare([]byte(token), s.secret) != 1 {
			s.log.Warn("unauthorized request", "remote_addr", r.RemoteAddr, "path", r.URL.Path)
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func extractToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return header[len("Bearer "):]
	}
	return ""
}

func (s *Server) requireSessionToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractSessionToken(r)
		if token == "" {
			s.log.Warn("SSE missing session token", "remote_addr", r.RemoteAddr)
			writeError(w, http.StatusUnauthorized, "unauthorized", "session token required")
			return
		}
		if err := s.signer.Validate(token, sessiontoken.PurposeSSE); err != nil {
			s.log.Warn("SSE invalid session token", "remote_addr", r.RemoteAddr, "reason", err.Error())
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired session token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func extractSessionToken(r *http.Request) string {
	if c, err := r.Cookie("fox_sse_token"); err == nil && c.Value != "" {
		return c.Value
	}
	return ""
}
