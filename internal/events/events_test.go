package events

import (
	"sync"
	"testing"
)

func TestEmitAndRecent(t *testing.T) {
	log := NewLog(5)
	log.Emit("provision", "fox-1", "started")
	log.Emit("destroy", "fox-2", "destroyed")

	got := log.Recent(10)
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	if got[0].Type != "destroy" {
		t.Errorf("latest event type = %q, want destroy", got[0].Type)
	}
	if got[1].Type != "provision" {
		t.Errorf("oldest event type = %q, want provision", got[1].Type)
	}
	if got[0].ID <= got[1].ID {
		t.Error("events should be newest-first")
	}
}

func TestRingWrap(t *testing.T) {
	log := NewLog(3)
	for i := 0; i < 7; i++ {
		log.Emitf("test", "", "event-%d", i)
	}
	got := log.Recent(10)
	if len(got) != 3 {
		t.Fatalf("expected 3 events after wrap, got %d", len(got))
	}
	if got[0].Message != "event-6" {
		t.Errorf("newest = %q, want event-6", got[0].Message)
	}
	if got[2].Message != "event-4" {
		t.Errorf("oldest = %q, want event-4", got[2].Message)
	}
}

func TestRecentEmpty(t *testing.T) {
	log := NewLog(10)
	got := log.Recent(5)
	if len(got) != 0 {
		t.Fatalf("expected 0 events, got %d", len(got))
	}
}

func TestConcurrentEmit(t *testing.T) {
	log := NewLog(50)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				log.Emit("test", "", "msg")
			}
		}()
	}
	wg.Wait()
	got := log.Recent(50)
	if len(got) != 50 {
		t.Fatalf("expected 50 events, got %d", len(got))
	}
}

func TestSequentialIDs(t *testing.T) {
	log := NewLog(10)
	log.Emit("a", "", "first")
	log.Emit("b", "", "second")
	got := log.Recent(2)
	if got[0].ID != 2 || got[1].ID != 1 {
		t.Errorf("IDs should be sequential: got %d, %d", got[0].ID, got[1].ID)
	}
}
