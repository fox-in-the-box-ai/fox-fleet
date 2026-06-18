package api

import (
	"fmt"
	"html"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func (s *Server) handleCloudProxy(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(s.cloudCfg.CookieName)
	if err != nil || c.Value == "" {
		http.Redirect(w, r, "/cloud/login", http.StatusSeeOther)
		return
	}

	sess, err := s.sessions.Validate(c.Value)
	if err != nil {
		http.Redirect(w, r, "/cloud/login", http.StatusSeeOther)
		return
	}

	u, err := s.users.Get(sess.UserID)
	if err != nil {
		s.log.Error("cloud proxy: user lookup failed", "username", sess.UserID, "error", err)
		http.Redirect(w, r, "/cloud/login", http.StatusSeeOther)
		return
	}

	if u.InstanceID == nil || *u.InstanceID == "" {
		s.serveCloud503(w, sess.UserID, "no instance assigned")
		return
	}

	inst, err := s.registry.Get(*u.InstanceID)
	if err != nil {
		s.log.Error("cloud proxy: instance not found", "instance_id", *u.InstanceID, "error", err)
		s.serveCloud503(w, *u.InstanceID, "instance not found")
		return
	}

	if inst.Status != "running" {
		s.serveCloud503(w, *u.InstanceID, inst.Status)
		return
	}

	target, err := url.Parse(fmt.Sprintf("http://localhost:%d", inst.Port))
	if err != nil {
		s.log.Error("cloud proxy: bad target URL", "instance_id", inst.ID, "port", inst.Port, "error", err)
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
			pr.Out.URL.Path = strings.TrimPrefix(r.URL.Path, "/cloud")
			if pr.Out.URL.Path == "" {
				pr.Out.URL.Path = "/"
			}
			pr.Out.URL.RawQuery = r.URL.RawQuery
			pr.Out.Host = target.Host
			if proxyPwd != "" {
				pr.Out.Header.Set("X-Fox-Auth", proxyPwd)
			}
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			s.log.Error("cloud proxy: backend unreachable", "instance_id", inst.ID, "error", err)
			s.serveCloud503(w, inst.ID, "unreachable")
		},
	}

	proxy.ServeHTTP(w, r)
}

func (s *Server) handleCloudRoot(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(s.cloudCfg.CookieName)
	if err != nil || c.Value == "" {
		http.Redirect(w, r, "/cloud/login", http.StatusSeeOther)
		return
	}

	if _, err := s.sessions.Validate(c.Value); err != nil {
		http.Redirect(w, r, "/cloud/login", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/cloud/", http.StatusSeeOther)
}

func (s *Server) serveCloud503(w http.ResponseWriter, instanceID, reason string) {
	setSecurityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src 'self'")
	w.WriteHeader(http.StatusServiceUnavailable)
	fmt.Fprintf(w, cloud503Page, html.EscapeString(instanceID), html.EscapeString(reason))
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
}

const cloud503Page = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Instance Unavailable - Fox Fleet</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;background:#1a1a2e;color:#e0e0e0;display:flex;align-items:center;justify-content:center;min-height:100vh}
.card{background:#16213e;border-radius:12px;padding:48px;max-width:440px;width:90%%;text-align:center;box-shadow:0 4px 24px rgba(0,0,0,.3)}
h1{font-size:1.5rem;margin-bottom:12px;color:#ff6b6b}
p{margin-bottom:8px;line-height:1.6;color:#a0a0b0}
.instance{font-family:monospace;background:#0f3460;padding:2px 8px;border-radius:4px}
.status{font-family:monospace;color:#ff6b6b}
.retry{display:inline-block;margin-top:24px;padding:10px 24px;background:#e94560;color:#fff;text-decoration:none;border-radius:6px;font-weight:500}
.retry:hover{background:#c73651}
</style>
</head>
<body>
<div class="card">
<h1>Instance Unavailable</h1>
<p>Your instance <span class="instance">%s</span> is currently <span class="status">%s</span>.</p>
<p>Please wait for it to start or contact your administrator.</p>
<a class="retry" href="">Retry</a>
</div>
</body>
</html>`
