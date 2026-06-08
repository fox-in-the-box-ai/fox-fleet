package api

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type metrics struct {
	requestsTotal  atomic.Int64
	requestsByPath sync.Map
	errorsTotal    atomic.Int64
	provisionTotal atomic.Int64
	sseConnections atomic.Int64
	startTime      time.Time
}

func newMetrics() *metrics {
	return &metrics{startTime: time.Now()}
}

func (m *metrics) incRequests(path string) {
	m.requestsTotal.Add(1)
	v, _ := m.requestsByPath.LoadOrStore(path, new(atomic.Int64))
	v.(*atomic.Int64).Add(1)
}

func (m *metrics) incProvisions() { m.provisionTotal.Add(1) }
func (m *metrics) sseConnect()    { m.sseConnections.Add(1) }
func (m *metrics) sseDisconnect() { m.sseConnections.Add(-1) }

func (m *metrics) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		fmt.Fprintf(w, "# HELP fox_control_requests_total Total API requests.\n")
		fmt.Fprintf(w, "# TYPE fox_control_requests_total counter\n")
		fmt.Fprintf(w, "fox_control_requests_total %d\n", m.requestsTotal.Load())

		fmt.Fprintf(w, "# HELP fox_control_errors_total Total API errors.\n")
		fmt.Fprintf(w, "# TYPE fox_control_errors_total counter\n")
		fmt.Fprintf(w, "fox_control_errors_total %d\n", m.errorsTotal.Load())

		fmt.Fprintf(w, "# HELP fox_control_provisions_total Total provision operations.\n")
		fmt.Fprintf(w, "# TYPE fox_control_provisions_total counter\n")
		fmt.Fprintf(w, "fox_control_provisions_total %d\n", m.provisionTotal.Load())

		fmt.Fprintf(w, "# HELP fox_control_sse_connections Current SSE connections.\n")
		fmt.Fprintf(w, "# TYPE fox_control_sse_connections gauge\n")
		fmt.Fprintf(w, "fox_control_sse_connections %d\n", m.sseConnections.Load())

		fmt.Fprintf(w, "# HELP fox_control_uptime_seconds Seconds since server start.\n")
		fmt.Fprintf(w, "# TYPE fox_control_uptime_seconds gauge\n")
		fmt.Fprintf(w, "fox_control_uptime_seconds %.0f\n", time.Since(m.startTime).Seconds())
	})
}

func metricsMiddleware(m *metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m.incRequests(r.URL.Path)
			next.ServeHTTP(w, r)
		})
	}
}
