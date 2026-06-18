package api

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
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
	if strings.HasPrefix(host, "[") {
		if i := strings.LastIndex(host, "]:"); i != -1 {
			return host[:i+1]
		}
		return host
	}
	if i := strings.LastIndex(host, ":"); i != -1 {
		return host[:i]
	}
	return host
}

func (s *Server) handleSubdomainRequest(w http.ResponseWriter, r *http.Request, slug string) {
	if r.URL.Path == "/login" {
		switch r.Method {
		case http.MethodGet:
			s.handleSubdomainLoginPage(w, r, slug)
		case http.MethodPost:
			if s.loginRL != nil {
				if ok, wait := s.loginRL.allow(); !ok {
					w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
					writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
					return
				}
			}
			s.handleSubdomainLogin(w, r, slug)
		default:
			setSecurityHeaders(w)
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET or POST")
		}
		return
	}
	if r.URL.Path == "/logout" && r.Method == http.MethodPost {
		s.handleCloudLogout(w, r)
		return
	}

	c, err := r.Cookie(s.cloudCfg.CookieName)
	if err != nil || c.Value == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	sess, err := s.sessions.Validate(c.Value)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if sess.UserID != slug {
		setSecurityHeaders(w)
		writeError(w, http.StatusForbidden, "forbidden", "session does not match subdomain")
		return
	}

	u, err := s.users.Get(slug)
	if err != nil {
		s.log.Error("subdomain proxy: user lookup failed", "slug", slug, "error", err)
		s.serveCloud503(w, slug, "user not found")
		return
	}

	if u.InstanceID == nil || *u.InstanceID == "" {
		s.serveCloud503(w, slug, "no instance assigned")
		return
	}

	inst, err := s.registry.Get(*u.InstanceID)
	if err != nil {
		s.log.Error("subdomain proxy: instance not found", "instance_id", *u.InstanceID, "error", err)
		s.serveCloud503(w, *u.InstanceID, "instance not found")
		return
	}

	if inst.Status != "running" {
		s.serveCloud503(w, *u.InstanceID, inst.Status)
		return
	}

	target, err := url.Parse(fmt.Sprintf("http://localhost:%d", inst.Port))
	if err != nil {
		s.log.Error("subdomain proxy: bad target URL", "instance_id", inst.ID, "port", inst.Port, "error", err)
		s.serveCloud503(w, inst.ID, "internal error")
		return
	}

	proxyPwd := inst.InstancePassword
	if proxyPwd == "" {
		proxyPwd = s.instPwd
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.SetXForwarded()
			pr.Out.URL.Path = r.URL.Path
			pr.Out.URL.RawQuery = r.URL.RawQuery
			pr.Out.Host = target.Host
			if proxyPwd != "" {
				pr.Out.Header.Set("X-Fox-Auth", proxyPwd)
			}
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			s.log.Error("subdomain proxy: backend unreachable", "instance_id", inst.ID, "error", err)
			s.serveCloud503(w, inst.ID, "unreachable")
		},
	}

	proxy.ServeHTTP(w, r)
}
