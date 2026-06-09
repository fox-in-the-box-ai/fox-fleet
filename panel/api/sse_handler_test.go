package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
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

type sseTestEnv struct {
	server *Server
	elog   *events.Log
	logBuf *bytes.Buffer
}

func newSSETestEnv(t *testing.T) *sseTestEnv {
	t.Helper()
	dir := t.TempDir()
	reg, err := registry.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reg.Close() })

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	elog := events.NewLog(100)

	srv := NewServer(Deps{
		Registry:     reg,
		Provisioner:  &fakeProvisioner{provisionCh: make(chan struct{}, 1)},
		Plugin:       &fakePlugin{},
		AdminSecret:  testSecret,
		InstancePwd:  "test-pwd",
		MaxInstances: 2,
		EventLog:     elog,
		SigningKey:   testSigningKey,
		Logger:       logger,
	})

	return &sseTestEnv{server: srv, elog: elog, logBuf: &logBuf}
}

type sessionResult struct {
	token  string
	cookie *http.Cookie
}

func getSession(t *testing.T, ts *httptest.Server) sessionResult {
	t.Helper()
	body := strings.NewReader(`{"purpose":"sse"}`)
	req, _ := http.NewRequest("POST", ts.URL+"/api/auth/session", body)
	req.Header.Set("Authorization", "Bearer "+testSecret)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("session endpoint returned %d", resp.StatusCode)
	}
	var data struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatal(err)
	}
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "fox_sse_token" {
			cookie = c
			break
		}
	}
	if cookie == nil {
		t.Fatal("Set-Cookie fox_sse_token not found in session response")
	}
	return sessionResult{token: data.Token, cookie: cookie}
}

func TestSSE_AuthRequired(t *testing.T) {
	env := newSSETestEnv(t)
	req := httptest.NewRequest("GET", "/api/events/stream", nil)
	w := httptest.NewRecorder()
	env.server.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestSSE_AdminSecretInQueryRejected(t *testing.T) {
	env := newSSETestEnv(t)

	ts := httptest.NewServer(env.server.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/events/stream?token="+testSecret, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for admin secret in query param, got %d", resp.StatusCode)
	}
}

func TestSSE_QueryParamSessionTokenAccepted(t *testing.T) {
	env := newSSETestEnv(t)

	ts := httptest.NewServer(env.server.Handler())
	defer ts.Close()

	sess := getSession(t, ts)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/events/stream?token="+sess.token, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for session token in query param, got %d", resp.StatusCode)
	}
}

func TestSSE_CookieAuth(t *testing.T) {
	env := newSSETestEnv(t)

	ts := httptest.NewServer(env.server.Handler())
	defer ts.Close()

	sess := getSession(t, ts)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/events/stream", nil)
	req.AddCookie(sess.cookie)
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
	env := newSSETestEnv(t)

	ts := httptest.NewServer(env.server.Handler())
	defer ts.Close()

	sess := getSession(t, ts)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/events/stream", nil)
	req.AddCookie(sess.cookie)
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

	env.elog.Emit("provision", "fox-test", "test event")

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
	env := newSSETestEnv(t)

	env.elog.Emit("a", "", "first")
	env.elog.Emit("b", "", "second")
	env.elog.Emit("c", "", "third")

	ts := httptest.NewServer(env.server.Handler())
	defer ts.Close()

	sess := getSession(t, ts)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/events/stream", nil)
	req.AddCookie(sess.cookie)
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

func TestSSE_TokenNotInLogs(t *testing.T) {
	env := newSSETestEnv(t)

	ts := httptest.NewServer(env.server.Handler())
	defer ts.Close()

	sess := getSession(t, ts)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/events/stream", nil)
	req.AddCookie(&http.Cookie{Name: "fox_sse_token", Value: "bogus-token"})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	logOutput := env.logBuf.String()
	if strings.Contains(logOutput, sess.token) {
		t.Fatal("session token found in log output")
	}
	if strings.Contains(logOutput, testSecret) {
		t.Fatal("admin secret found in log output")
	}
	if strings.Contains(logOutput, "bogus-token") {
		t.Fatal("rejected token value found in log output")
	}
}

func TestSessionEndpoint(t *testing.T) {
	env := newSSETestEnv(t)

	ts := httptest.NewServer(env.server.Handler())
	defer ts.Close()

	t.Run("success", func(t *testing.T) {
		sess := getSession(t, ts)
		if sess.token == "" {
			t.Fatal("empty token returned")
		}
	})

	t.Run("sets_httponly_cookie", func(t *testing.T) {
		sess := getSession(t, ts)
		if !sess.cookie.HttpOnly {
			t.Fatal("expected HttpOnly flag on fox_sse_token cookie")
		}
		if sess.cookie.SameSite != http.SameSiteStrictMode {
			t.Fatalf("expected SameSite=Strict, got %v", sess.cookie.SameSite)
		}
		if sess.cookie.Path != "/api/events" {
			t.Fatalf("expected Path=/api/events, got %q", sess.cookie.Path)
		}
	})

	t.Run("no_auth", func(t *testing.T) {
		body := strings.NewReader(`{"purpose":"sse"}`)
		req, _ := http.NewRequest("POST", ts.URL+"/api/auth/session", body)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("bad_purpose", func(t *testing.T) {
		body := strings.NewReader(`{"purpose":"admin"}`)
		req, _ := http.NewRequest("POST", ts.URL+"/api/auth/session", body)
		req.Header.Set("Authorization", "Bearer "+testSecret)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})
}
