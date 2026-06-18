package api

import (
	"net/http"
	"strings"
)

// hostDispatcher routes requests by Host header when cloud mode is enabled:
//   - base domain → existing mux (admin panel, API, login)
//   - <slug>.base-domain → subdomain proxy (Fox instance)
func (s *Server) hostDispatcher() http.Handler {
	baseDomain := s.cloudCfg.Domain
	suffix := "." + baseDomain

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := stripPort(r.Host)

		if host == baseDomain || !strings.HasSuffix(host, suffix) {
			s.mux.ServeHTTP(w, r)
			return
		}

		slug := strings.TrimSuffix(host, suffix)
		if slug == "" || strings.Contains(slug, ".") {
			setSecurityHeaders(w)
			writeError(w, http.StatusBadRequest, "bad_request", "invalid subdomain")
			return
		}

		s.handleSubdomainRequest(w, r, slug)
	})
}

func stripPort(host string) string {
	if i := strings.LastIndex(host, ":"); i != -1 {
		return host[:i]
	}
	return host
}

// handleSubdomainRequest proxies a request to the Fox instance owned by slug.
// Stub: returns 503 until T-011 implements the full subdomain proxy.
func (s *Server) handleSubdomainRequest(w http.ResponseWriter, r *http.Request, slug string) {
	setSecurityHeaders(w)
	s.serveCloud503(w, slug, "not yet available")
}
