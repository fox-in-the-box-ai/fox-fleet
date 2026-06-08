package registry

import (
	"fmt"
	"path/filepath"
	"testing"
)

func benchDB(b *testing.B, count int) *Registry {
	b.Helper()
	path := filepath.Join(b.TempDir(), "bench.db")
	reg, err := Open(path)
	if err != nil {
		b.Fatalf("Open: %v", err)
	}
	b.Cleanup(func() { reg.Close() })
	for i := 0; i < count; i++ {
		inst := Instance{
			ID:          fmt.Sprintf("inst-%04d", i),
			ImageDigest: "sha256:abc123",
			Port:        9000 + i,
			DataDir:     fmt.Sprintf("/data/inst-%04d", i),
			Status:      "running",
			CreatedAt:   "2026-01-01T00:00:00Z",
		}
		if err := reg.Create(inst); err != nil {
			b.Fatalf("seed Create: %v", err)
		}
	}
	return reg
}

func BenchmarkRegistryList(b *testing.B) {
	reg := benchDB(b, 100)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := reg.List(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRegistryGet(b *testing.B) {
	reg := benchDB(b, 100)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := reg.Get("inst-0050"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRegistryCreate(b *testing.B) {
	reg := benchDB(b, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		inst := Instance{
			ID:          fmt.Sprintf("bench-%08d", i),
			ImageDigest: "sha256:abc123",
			Port:        9000 + i,
			DataDir:     fmt.Sprintf("/data/bench-%08d", i),
			Status:      "running",
			CreatedAt:   "2026-01-01T00:00:00Z",
		}
		if err := reg.Create(inst); err != nil {
			b.Fatal(err)
		}
	}
}
