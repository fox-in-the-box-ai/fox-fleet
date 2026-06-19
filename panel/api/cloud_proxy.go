package api

import (
	"fmt"
	"html"
	"net/http"
)

func (s *Server) handleLegacyCloudRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
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

	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
}

func (s *Server) serveCloud503(w http.ResponseWriter, instanceID, reason string) {
	setSecurityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; img-src 'self'")
	w.WriteHeader(http.StatusServiceUnavailable)
	fmt.Fprintf(w, cloud503Page, html.EscapeString(instanceID), html.EscapeString(reason))
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
}

// cloud503Page colors sourced from design-system v0.1.0 tokens.
const cloud503Page = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Instance Unavailable - Fox Fleet</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Manrope:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Manrope',-apple-system,BlinkMacSystemFont,'Segoe UI',system-ui,sans-serif;background:#0D0D1A;color:#FFF8DC;display:flex;align-items:center;justify-content:center;min-height:100vh}
.card{background:#141425;border-radius:12px;padding:48px;max-width:440px;width:90%%;text-align:center;box-shadow:0 4px 24px rgba(0,0,0,.4)}
h1{font-size:1.5rem;margin-bottom:12px;color:#EF5350;font-weight:700}
p{margin-bottom:8px;line-height:1.6;color:#C0B6A2}
.instance{font-family:'JetBrains Mono',ui-monospace,monospace;background:#1A1A2E;padding:2px 8px;border-radius:4px}
.status{font-family:'JetBrains Mono',ui-monospace,monospace;color:#EF5350}
.retry{display:inline-block;margin-top:24px;padding:10px 24px;background:#EA580C;color:#fff;text-decoration:none;border-radius:8px;font-weight:600;transition:background .2s}
.retry:hover{background:#C2410C}
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
