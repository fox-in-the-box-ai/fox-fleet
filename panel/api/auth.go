package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
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
	if r.URL.Path == "/api/events/stream" {
		return r.URL.Query().Get("token")
	}
	return ""
}
