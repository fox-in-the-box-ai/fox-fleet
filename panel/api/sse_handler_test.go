package api

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/events"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
)

func newSSETestEnv(t *testing.T) (*Server, *events.Log) {
	t.Helper()
	dir := t.TempDir()
	reg, err := registry.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Close() })

	elog := events.NewLog(100)

	srv := NewServer(Deps{
		Registry:     reg,
		Provisioner:  &fakeProvisioner{provisionCh: make(chan struct{}, 1)},
		Plugin:       &fakePlugin{},
		AdminSecret:  testSecret,
		InstancePwd:  "test-pwd",
		MaxInstances: 2,
		EventLog:     elog,
	})

	return srv, elog
}

func TestSSE_AuthRequired(t *testing.T) {
	srv, _ := newSSETestEnv(t)
	req := httptest.NewRequest("GET", "/api/events/stream", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestSSE_QueryParamAuth(t *testing.T) {
	srv, _ := newSSETestEnv(t)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/events/stream?token="+testSecret, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}

	scanner := bufio.NewScanner(resp.Body)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) >= 3 {
			break
		}
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "event: connected") {
		t.Errorf("expected 'event: connected' in initial output, got:\n%s", joined)
	}
}

func TestSSE_StreamsLiveEvents(t *testing.T) {
	srv, elog := newSSETestEnv(t)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/events/stream?token="+testSecret, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "event: connected") {
			break
		}
	}

	elog.Emit("provision", "fox-test", "test event")

	var found bool
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "fox-test") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("did not receive live event on SSE stream")
	}
}

func TestSSE_LastEventIDReplay(t *testing.T) {
	srv, elog := newSSETestEnv(t)

	elog.Emit("a", "", "first")
	elog.Emit("b", "", "second")
	elog.Emit("c", "", "third")

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/events/stream?token="+testSecret, nil)
	req.Header.Set("Last-Event-ID", "1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	var dataLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") && !strings.Contains(line, "data: {}") {
			dataLines = append(dataLines, line)
		}
		if len(dataLines) >= 2 {
			break
		}
	}

	if len(dataLines) < 2 {
		t.Fatalf("expected at least 2 replayed events, got %d", len(dataLines))
	}
	if !strings.Contains(dataLines[0], "second") {
		t.Errorf("first replay should contain 'second', got: %s", dataLines[0])
	}
	if !strings.Contains(dataLines[1], "third") {
		t.Errorf("second replay should contain 'third', got: %s", dataLines[1])
	}
}
