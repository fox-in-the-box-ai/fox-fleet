package events

import (
	"fmt"
	"testing"
)

func BenchmarkEmit(b *testing.B) {
	log := NewLog(200)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		log.Emit("bench", "inst-1", fmt.Sprintf("event-%d", i))
	}
}

func BenchmarkEmitParallel(b *testing.B) {
	log := NewLog(200)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			log.Emit("bench", "inst-1", fmt.Sprintf("event-%d", i))
			i++
		}
	})
}

func BenchmarkRecent(b *testing.B) {
	log := NewLog(200)
	for i := 0; i < 200; i++ {
		log.Emit("seed", "inst-1", fmt.Sprintf("event-%d", i))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		log.Recent(50)
	}
}

func BenchmarkSinceID(b *testing.B) {
	log := NewLog(200)
	for i := 0; i < 200; i++ {
		log.Emit("seed", "inst-1", fmt.Sprintf("event-%d", i))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		log.SinceID(180)
	}
}
