package api

import (
	"encoding/json"
	"net/http"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/sessiontoken"
)

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	var req struct {
		Purpose string `json:"purpose"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
		return
	}
	if req.Purpose != "sse" {
		writeError(w, http.StatusBadRequest, "bad_request", "unsupported purpose")
		return
	}

	token, expiresAt, err := s.signer.Generate(sessiontoken.PurposeSSE, s.sessionTTL)
	if err != nil {
		s.log.Error("generate session token", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "cannot generate session token")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "fox_sse_token",
		Value:    token,
		Path:     "/api/events",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   r.TLS != nil,
	})

	s.log.Info("session token issued",
		"purpose", req.Purpose,
		"expires_at", expiresAt.Format("2006-01-02T15:04:05Z"),
		"remote_addr", r.RemoteAddr,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"token":      token,
		"expires_at": expiresAt.Format("2006-01-02T15:04:05Z"),
		"purpose":    req.Purpose,
	}); err != nil {
		s.log.Error("encode session response", "error", err)
	}
}
