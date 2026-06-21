package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/cloud"
)

func (s *Server) handleSubdomainLogin(w http.ResponseWriter, r *http.Request, slug string) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCreateBody)
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if req.Username == "" {
		req.Username = slug
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "password is required")
		return
	}
	if len(req.Password) > 72 {
		writeError(w, http.StatusBadRequest, "bad_request", "password too long")
		return
	}

	if req.Username != slug {
		writeError(w, http.StatusForbidden, "forbidden", "username does not match subdomain")
		return
	}

	_, err := s.users.Authenticate(req.Username, req.Password)
	if errors.Is(err, cloud.ErrInvalidCredentials) {
		s.log.Warn("subdomain login failed: invalid credentials", "slug", slug)
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
		return
	}
	if err != nil {
		s.log.Error("subdomain login failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "login failed")
		return
	}

	token, _, err := s.sessions.Create(req.Username, s.cloudCfg.SessionTTL)
	if err != nil {
		s.log.Error("subdomain session create failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "login failed")
		return
	}

	s.log.Info("subdomain login succeeded", "slug", slug)
	http.SetCookie(w, s.sessionCookie(token, s.cloudCfg.SessionTTL))
	writeJSON(w, http.StatusOK, map[string]string{"username": req.Username})
}

func (s *Server) handleSubdomainLoginPage(w http.ResponseWriter, r *http.Request, slug string) {
	// Clean up any stale session cookie (valid sessions are handled upstream).
	c, err := r.Cookie(s.cloudCfg.CookieName)
	if err == nil && c.Value != "" {
		_ = s.sessions.Delete(c.Value)
		http.SetCookie(w, s.sessionCookie("", -time.Hour))
	}

	setSecurityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; img-src 'self'; connect-src 'self'; form-action 'self'")
	_, _ = w.Write([]byte(subdomainLoginPage))
}

// subdomainLoginPage colors sourced from design-system v0.1.0 tokens.
// Password-only: username is inferred from the subdomain (slug == username).
const subdomainLoginPage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sign In - Fox Fleet</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Manrope:wght@400;500;600;700&display=swap" rel="stylesheet">
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Manrope',-apple-system,BlinkMacSystemFont,'Segoe UI',system-ui,sans-serif;background:#0D0D1A;color:#FFF8DC;display:flex;align-items:center;justify-content:center;min-height:100vh}
.card{background:#141425;border-radius:12px;padding:48px;max-width:400px;width:90%;box-shadow:0 4px 24px rgba(0,0,0,.4)}
.logo{text-align:center;margin-bottom:32px}
.logo svg{width:48px;height:48px}
.logo h1{font-size:1.25rem;margin-top:12px;color:#fff;font-weight:700}
.logo p{font-size:.875rem;color:#C0B6A2;margin-top:4px}
label{display:block;font-size:.875rem;color:#C0B6A2;margin-bottom:6px}
input{width:100%;padding:10px 14px;background:#1A1A2E;border:1px solid #2A2A45;border-radius:8px;color:#FFF8DC;font-size:1rem;font-family:inherit;margin-bottom:16px;outline:none;transition:border-color .2s}
input:focus{border-color:#EA580C}
button{width:100%;padding:12px;background:#EA580C;color:#fff;border:none;border-radius:8px;font-size:1rem;font-weight:600;font-family:inherit;cursor:pointer;transition:background .2s}
button:hover{background:#C2410C}
button:disabled{opacity:.6;cursor:not-allowed}
.error{background:rgba(198,40,40,.12);border:1px solid rgba(198,40,40,.3);border-radius:8px;padding:10px 14px;margin-bottom:16px;font-size:.875rem;color:#EF5350;display:none}
</style>
</head>
<body>
<div class="card">
<div class="logo">
<svg viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
<circle cx="24" cy="24" r="22" stroke="#EA580C" stroke-width="2" fill="#1A1A2E"/>
<text x="24" y="30" text-anchor="middle" fill="#F97316" font-size="20" font-weight="bold" font-family="sans-serif">F</text>
</svg>
<h1>Fox Fleet</h1>
<p id="subtitle">Sign in to access your instance</p>
</div>
<div class="error" id="error"></div>
<form id="loginForm" autocomplete="on">
<input type="hidden" id="username" name="username" autocomplete="username">
<label for="password">Password</label>
<input type="password" id="password" name="password" required autofocus autocomplete="current-password">
<button type="submit" id="submitBtn">Sign In</button>
</form>
</div>
<script>
(function(){
var slug=location.hostname.split(".")[0];
document.getElementById("username").value=slug;
document.getElementById("subtitle").textContent="Sign in as "+slug;
var form=document.getElementById("loginForm");
var err=document.getElementById("error");
var btn=document.getElementById("submitBtn");
form.addEventListener("submit",function(e){
e.preventDefault();
err.style.display="none";
btn.disabled=true;
btn.textContent="Signing in…";
var body=JSON.stringify({password:document.getElementById("password").value});
fetch("/login",{method:"POST",headers:{"Content-Type":"application/json"},body:body,credentials:"same-origin"})
.then(function(r){
if(r.ok){window.location.href="/";return}
return r.json().catch(function(){return{}}).then(function(d){throw new Error(d.message||"Invalid credentials")});
})
.catch(function(ex){
err.textContent=ex.message||"Invalid credentials";
err.style.display="block";
btn.disabled=false;
btn.textContent="Sign In";
});
});
})();
</script>
</body>
</html>`
