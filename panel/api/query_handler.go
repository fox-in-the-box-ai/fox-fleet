package api

import (
	"io"
	"net/http"
	"strings"
)

const maxQueryBody = 4096

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	if s.dpURL == "" {
		writeError(w, http.StatusServiceUnavailable, "no_data_plane", "data plane is not configured — set data_plane.enabled = true in config")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxQueryBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "cannot read request body")
		return
	}

	target := strings.TrimRight(s.dpURL, "/") + "/v1/query"
	proxyReq, err := http.NewRequestWithContext(r.Context(), "POST", target, strings.NewReader(string(body)))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "cannot create proxy request to data plane")
		return
	}
	proxyReq.Header.Set("Content-Type", "application/json")
	proxyReq.Header.Set("Authorization", "Bearer "+string(s.secret))

	resp, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "data_plane_error",
			"cannot reach data plane — verify it is running and data_plane.listen is correct")
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 1<<20))
}
