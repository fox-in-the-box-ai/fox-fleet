package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			s.log.Warn("unauthorized request", "remote_addr", r.RemoteAddr, "path", r.URL.Path)
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid Authorization header")
			return
		}
		token := header[len("Bearer "):]
		if subtle.ConstantTimeCompare([]byte(token), s.secret) != 1 {
			s.log.Warn("unauthorized request", "remote_addr", r.RemoteAddr, "path", r.URL.Path)
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid Authorization header")
			return
		}
		next.ServeHTTP(w, r)
	})
}
