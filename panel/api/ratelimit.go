package api

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

type rateLimiter struct {
	mu       sync.Mutex
	tokens   float64
	max      float64
	rate     float64
	lastTick time.Time
}

func newRateLimiter(perMinute int) *rateLimiter {
	max := float64(perMinute)
	return &rateLimiter{
		tokens:   max,
		max:      max,
		rate:     max / 60.0,
		lastTick: time.Now(),
	}
}

func (rl *rateLimiter) allow() (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastTick).Seconds()
	rl.lastTick = now
	rl.tokens += elapsed * rl.rate
	if rl.tokens > rl.max {
		rl.tokens = rl.max
	}

	if rl.tokens < 1 {
		wait := time.Duration((1 - rl.tokens) / rl.rate * float64(time.Second))
		return false, wait
	}
	rl.tokens--
	return true, 0
}

func rateLimitMiddleware(rl *rateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok, wait := rl.allow()
			if !ok {
				w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
				writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
