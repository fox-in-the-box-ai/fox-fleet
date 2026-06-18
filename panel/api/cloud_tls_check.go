package api

import (
	"net/http"
	"strings"
)

// handleTLSCheck responds to Caddy's on_demand_tls ask endpoint.
// Returns 200 if the subdomain belongs to a registered user with an
// assigned instance; 403 otherwise. Caddy interprets non-2xx as "do
// not issue a certificate for this domain."
func (s *Server) handleTLSCheck(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	if domain == "" {
		writeError(w, http.StatusForbidden, "forbidden", "missing domain parameter")
		return
	}

	baseDomain := s.cloudCfg.Domain
	suffix := "." + baseDomain
	if !strings.HasSuffix(domain, suffix) {
		writeError(w, http.StatusForbidden, "forbidden", "domain not under base domain")
		return
	}

	slug := strings.TrimSuffix(domain, suffix)
	if slug == "" || strings.Contains(slug, ".") {
		writeError(w, http.StatusForbidden, "forbidden", "invalid subdomain")
		return
	}

	u, err := s.users.Get(slug)
	if err != nil {
		writeError(w, http.StatusForbidden, "forbidden", "forbidden")
		return
	}

	if u.InstanceID == nil || *u.InstanceID == "" {
		writeError(w, http.StatusForbidden, "forbidden", "forbidden")
		return
	}

	w.WriteHeader(http.StatusOK)
}
