package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

func (s *Server) handleEventsStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal_error", "streaming not supported")
		return
	}

	if s.events == nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "event log not configured")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	fmt.Fprint(w, "retry: 3000\nevent: connected\ndata: {}\n\n")
	flusher.Flush()

	if lastIDStr := r.Header.Get("Last-Event-ID"); lastIDStr != "" {
		if lastID, err := strconv.ParseUint(lastIDStr, 10, 64); err == nil {
			for _, e := range s.events.SinceID(lastID) {
				data, _ := json.Marshal(e)
				fmt.Fprintf(w, "id: %d\ndata: %s\n\n", e.ID, data)
			}
			flusher.Flush()
		}
	}

	ch, cancel := s.events.Subscribe()
	defer cancel()

	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-ch:
			data, _ := json.Marshal(e)
			fmt.Fprintf(w, "id: %d\ndata: %s\n\n", e.ID, data)
			flusher.Flush()
		}
	}
}
