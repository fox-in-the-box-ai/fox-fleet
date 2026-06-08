package events

import (
	"fmt"
	"log/slog"
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
	mu    sync.RWMutex
	buf   []Event
	cap   int
	pos   int
	full  bool
	seq   atomic.Uint64
	store *Store

	subMu  sync.Mutex
	subs   map[uint64]chan Event
	subSeq atomic.Uint64

	webhooks *WebhookDispatcher
}

func NewLog(capacity int) *Log {
	if capacity < 1 {
		capacity = 200
	}
	return &Log{buf: make([]Event, capacity), cap: capacity}
}

func NewPersistentLog(capacity int, store *Store) *Log {
	l := NewLog(capacity)
	l.store = store
	recent, err := store.Recent(capacity)
	if err != nil {
		slog.Warn("failed to load recent events from store", "error", err)
		return l
	}
	for i := len(recent) - 1; i >= 0; i-- {
		e := recent[i]
		l.buf[l.pos] = e
		l.pos = (l.pos + 1) % l.cap
		if l.pos == 0 {
			l.full = true
		}
		if e.ID > l.seq.Load() {
			l.seq.Store(e.ID)
		}
	}
	return l
}

func (l *Log) Emit(typ, instance, message string) {
	e := Event{
		Type:      typ,
		Instance:  instance,
		Message:   message,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if l.store != nil {
		id, err := l.store.Insert(e)
		if err != nil {
			slog.Warn("failed to persist event", "error", err)
			e.ID = l.seq.Add(1)
		} else {
			e.ID = id
			l.seq.Store(id)
		}
	} else {
		e.ID = l.seq.Add(1)
	}

	l.mu.Lock()
	l.buf[l.pos] = e
	l.pos = (l.pos + 1) % l.cap
	if l.pos == 0 {
		l.full = true
	}
	l.mu.Unlock()

	l.subMu.Lock()
	for _, ch := range l.subs {
		select {
		case ch <- e:
		default:
		}
	}
	l.subMu.Unlock()

	if l.webhooks != nil {
		l.webhooks.Dispatch(e)
	}
}

func (l *Log) Emitf(typ, instance, format string, args ...any) {
	l.Emit(typ, instance, fmt.Sprintf(format, args...))
}

func (l *Log) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 16)
	id := l.subSeq.Add(1)
	l.subMu.Lock()
	if l.subs == nil {
		l.subs = make(map[uint64]chan Event)
	}
	l.subs[id] = ch
	l.subMu.Unlock()
	return ch, func() {
		l.subMu.Lock()
		delete(l.subs, id)
		l.subMu.Unlock()
	}
}

func (l *Log) SinceID(lastID uint64) []Event {
	l.mu.RLock()
	defer l.mu.RUnlock()

	size := l.pos
	if l.full {
		size = l.cap
	}
	var out []Event
	for i := 0; i < size; i++ {
		idx := (l.pos - size + i + l.cap) % l.cap
		e := l.buf[idx]
		if e.ID > lastID {
			out = append(out, e)
		}
	}
	return out
}

func (l *Log) Store() *Store {
	return l.store
}

func (l *Log) SetWebhooks(wd *WebhookDispatcher) {
	l.webhooks = wd
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
