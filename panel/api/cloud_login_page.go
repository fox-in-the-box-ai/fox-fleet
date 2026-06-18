package api

import (
	"net/http"
)

func (s *Server) handleCloudLoginPage(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(s.cloudCfg.CookieName)
	if err == nil && c.Value != "" {
		if _, err := s.sessions.Validate(c.Value); err == nil {
			http.Redirect(w, r, "/admin/", http.StatusSeeOther)
			return
		}
	}

	setSecurityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src 'self'; connect-src 'self'; form-action 'self'")
	_, _ = w.Write([]byte(cloudLoginPage))
}

const cloudLoginPage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sign In - Fox Fleet</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;background:#1a1a2e;color:#e0e0e0;display:flex;align-items:center;justify-content:center;min-height:100vh}
.card{background:#16213e;border-radius:12px;padding:48px;max-width:400px;width:90%;box-shadow:0 4px 24px rgba(0,0,0,.3)}
.logo{text-align:center;margin-bottom:32px}
.logo svg{width:48px;height:48px}
.logo h1{font-size:1.25rem;margin-top:12px;color:#fff}
.logo p{font-size:.875rem;color:#a0a0b0;margin-top:4px}
label{display:block;font-size:.875rem;color:#a0a0b0;margin-bottom:6px}
input{width:100%;padding:10px 14px;background:#0f3460;border:1px solid #1a4a8a;border-radius:6px;color:#e0e0e0;font-size:1rem;margin-bottom:16px;outline:none;transition:border-color .2s}
input:focus{border-color:#e94560}
button{width:100%;padding:12px;background:#e94560;color:#fff;border:none;border-radius:6px;font-size:1rem;font-weight:600;cursor:pointer;transition:background .2s}
button:hover{background:#c73651}
button:disabled{opacity:.6;cursor:not-allowed}
.error{background:#3d1f2f;border:1px solid #e94560;border-radius:6px;padding:10px 14px;margin-bottom:16px;font-size:.875rem;color:#ff6b6b;display:none}
</style>
</head>
<body>
<div class="card">
<div class="logo">
<svg viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
<circle cx="24" cy="24" r="22" stroke="#e94560" stroke-width="2" fill="#0f3460"/>
<text x="24" y="30" text-anchor="middle" fill="#e94560" font-size="20" font-weight="bold" font-family="sans-serif">F</text>
</svg>
<h1>Fox Fleet</h1>
<p>Sign in to access your instance</p>
</div>
<div class="error" id="error"></div>
<form id="loginForm" autocomplete="on">
<label for="username">Username</label>
<input type="text" id="username" name="username" required autofocus autocomplete="username">
<label for="password">Password</label>
<input type="password" id="password" name="password" required autocomplete="current-password">
<button type="submit" id="submitBtn">Sign In</button>
</form>
</div>
<script>
(function(){
var form=document.getElementById("loginForm");
var err=document.getElementById("error");
var btn=document.getElementById("submitBtn");
form.addEventListener("submit",function(e){
e.preventDefault();
err.style.display="none";
btn.disabled=true;
btn.textContent="Signing in…";
var body=JSON.stringify({
username:document.getElementById("username").value,
password:document.getElementById("password").value
});
fetch("/cloud/login",{method:"POST",headers:{"Content-Type":"application/json"},body:body,credentials:"same-origin"})
.then(function(r){
if(r.ok){window.location.href="/admin/";return}
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
