package api

import (
	"fmt"
	"net/http/httptest"
	"testing"
)

func BenchmarkWriteJSONList(b *testing.B) {
	items := make([]instanceItem, 20)
	for i := range items {
		items[i] = instanceItem{
			ID:        fmt.Sprintf("inst-%04d", i),
			Status:    "running",
			Port:      9000 + i,
			CreatedAt: "2026-01-01T00:00:00Z",
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		w := httptest.NewRecorder()
		writeJSON(w, 200, items)
	}
}

func BenchmarkWriteJSONDetail(b *testing.B) {
	detail := instanceDetail{
		ID:        "inst-0001",
		Status:    "running",
		Port:      9001,
		CreatedAt: "2026-01-01T00:00:00Z",
		Logs:      "log line 1\nlog line 2\n",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		w := httptest.NewRecorder()
		writeJSON(w, 200, detail)
	}
}
