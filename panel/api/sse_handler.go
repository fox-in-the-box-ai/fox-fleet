package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func (s *Server) handleEventsStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal_error", "streaming is not supported by this HTTP transport")
		return
	}

	if s.events == nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "event log is not configured on this server")
		return
	}

	rc := http.NewResponseController(w)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Referrer-Policy", "no-referrer")

	_ = rc.SetWriteDeadline(time.Now().Add(30 * time.Second))
	fmt.Fprint(w, "retry: 3000\nevent: connected\ndata: {}\n\n")
	flusher.Flush()

	// Subscribe before replay to close the race window — any event
	// emitted between SinceID and the live loop lands in ch.
	ch, cancel := s.events.Subscribe()
	defer cancel()
	s.metrics.sseConnect()
	defer s.metrics.sseDisconnect()

	var highID uint64
	if lastIDStr := r.Header.Get("Last-Event-ID"); lastIDStr != "" {
		if lastID, err := strconv.ParseUint(lastIDStr, 10, 64); err == nil {
			for _, e := range s.events.SinceID(lastID) {
				data, _ := json.Marshal(e)
				if _, err := fmt.Fprintf(w, "id: %d\ndata: %s\n\n", e.ID, data); err != nil {
					return
				}
				if e.ID > highID {
					highID = e.ID
				}
			}
			_ = rc.SetWriteDeadline(time.Now().Add(30 * time.Second))
			flusher.Flush()
		}
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-ch:
			if e.ID <= highID {
				continue
			}
			data, _ := json.Marshal(e)
			_ = rc.SetWriteDeadline(time.Now().Add(30 * time.Second))
			if _, err := fmt.Fprintf(w, "id: %d\ndata: %s\n\n", e.ID, data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
