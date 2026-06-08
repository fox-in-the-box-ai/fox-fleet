package mock_llm

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

type Server struct {
	ln       net.Listener
	srv      *http.Server
	failNext atomic.Bool
	serveErr atomic.Pointer[error]
	done     chan struct{}
}

func Start() (*Server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("mock_llm: listen: %w", err)
	}

	s := &Server{ln: ln}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", s.handleChat)
	mux.HandleFunc("/__control/fail", s.handleFail)

	s.srv = &http.Server{Handler: mux}
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.serveErr.Store(&err)
		}
	}()

	return s, nil
}

func (s *Server) Port() int {
	return s.ln.Addr().(*net.TCPAddr).Port
}

func (s *Server) ContainerURL() string {
	return fmt.Sprintf("http://host.docker.internal:%d/v1", s.Port())
}

func (s *Server) SetFailNext() {
	s.failNext.Store(true)
}

func (s *Server) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s.srv.Shutdown(ctx)
	<-s.done
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if s.failNext.CompareAndSwap(true, false) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"message":"mock provider error","type":"server_error"}}`))
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	tokens := []string{"Hello", " from", " mock"}
	for _, tok := range tokens {
		fmt.Fprintf(w, "data: {\"id\":\"mock\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":%q}}]}\n\n", tok)
		flusher.Flush()
	}

	fmt.Fprintf(w, "data: {\"id\":\"mock\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":3,\"total_tokens\":8}}\n\n")
	flusher.Flush()

	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (s *Server) handleFail(w http.ResponseWriter, r *http.Request) {
	s.failNext.Store(true)
	w.WriteHeader(http.StatusNoContent)
}
