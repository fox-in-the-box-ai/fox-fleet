package events

import (
	"sync"
	"testing"
	"time"
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

func TestSubscribeReceivesEvents(t *testing.T) {
	log := NewLog(10)
	ch, cancel := log.Subscribe()
	defer cancel()

	log.Emit("provision", "fox-1", "started")

	select {
	case e := <-ch:
		if e.Type != "provision" || e.Instance != "fox-1" {
			t.Errorf("got type=%q instance=%q, want provision/fox-1", e.Type, e.Instance)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive event within timeout")
	}
}

func TestSubscribeCancelStopsDelivery(t *testing.T) {
	log := NewLog(10)
	ch, cancel := log.Subscribe()
	cancel()

	log.Emit("provision", "fox-1", "after cancel")

	select {
	case <-ch:
		t.Fatal("cancelled subscriber should not receive events")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestSubscribeSlowSubscriberDoesNotBlock(t *testing.T) {
	log := NewLog(100)
	ch, cancel := log.Subscribe()
	defer cancel()

	for i := 0; i < 32; i++ {
		log.Emit("test", "", "flood")
	}

	received := 0
	for {
		select {
		case <-ch:
			received++
		default:
			goto done
		}
	}
done:
	if received != 16 {
		t.Errorf("expected 16 events (channel buffer), got %d", received)
	}
}

func TestConcurrentSubscribeEmit(t *testing.T) {
	log := NewLog(100)
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, cancel := log.Subscribe()
			defer cancel()
			for j := 0; j < 10; j++ {
				select {
				case <-ch:
				case <-time.After(time.Second):
				}
			}
		}()
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Emit("test", "", "concurrent")
		}()
	}

	wg.Wait()
}

func TestSinceID(t *testing.T) {
	log := NewLog(10)
	log.Emit("a", "", "first")
	log.Emit("b", "", "second")
	log.Emit("c", "", "third")

	got := log.SinceID(1)
	if len(got) != 2 {
		t.Fatalf("expected 2 events since ID 1, got %d", len(got))
	}
	if got[0].Message != "second" {
		t.Errorf("first result = %q, want second", got[0].Message)
	}
	if got[1].Message != "third" {
		t.Errorf("second result = %q, want third", got[1].Message)
	}
}

func TestSinceIDAfterWrap(t *testing.T) {
	log := NewLog(3)
	for i := 0; i < 7; i++ {
		log.Emitf("test", "", "event-%d", i)
	}
	got := log.SinceID(5)
	if len(got) != 2 {
		t.Fatalf("expected 2 events since ID 5, got %d", len(got))
	}
	if got[0].Message != "event-5" {
		t.Errorf("first = %q, want event-5", got[0].Message)
	}
}
