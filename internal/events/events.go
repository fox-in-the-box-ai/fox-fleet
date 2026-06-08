package events

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type Event struct {
	ID        uint64 `json:"id"`
	Type      string `json:"type"`
	Instance  string `json:"instance,omitempty"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

type Log struct {
	mu   sync.RWMutex
	buf  []Event
	cap  int
	pos  int
	full bool
	seq  atomic.Uint64
}

func NewLog(capacity int) *Log {
	if capacity < 1 {
		capacity = 200
	}
	return &Log{buf: make([]Event, capacity), cap: capacity}
}

func (l *Log) Emit(typ, instance, message string) {
	e := Event{
		ID:        l.seq.Add(1),
		Type:      typ,
		Instance:  instance,
		Message:   message,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	l.mu.Lock()
	l.buf[l.pos] = e
	l.pos = (l.pos + 1) % l.cap
	if l.pos == 0 {
		l.full = true
	}
	l.mu.Unlock()
}

func (l *Log) Emitf(typ, instance, format string, args ...any) {
	l.Emit(typ, instance, fmt.Sprintf(format, args...))
}

func (l *Log) Recent(n int) []Event {
	l.mu.RLock()
	defer l.mu.RUnlock()

	size := l.pos
	if l.full {
		size = l.cap
	}
	if n <= 0 || n > size {
		n = size
	}
	out := make([]Event, n)
	for i := 0; i < n; i++ {
		idx := (l.pos - n + i + l.cap) % l.cap
		out[n-1-i] = l.buf[idx]
	}
	return out
}
