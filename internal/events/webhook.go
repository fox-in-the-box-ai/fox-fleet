package events

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type WebhookTarget struct {
	URL    string
	Events []string
	Secret string
}

type WebhookDispatcher struct {
	targets []WebhookTarget
	client  *http.Client
	limiter map[string]*webhookLimiter
	mu      sync.Mutex
}

type webhookLimiter struct {
	tokens   float64
	max      float64
	rate     float64
	lastTick time.Time
}

func NewWebhookDispatcher(targets []WebhookTarget) *WebhookDispatcher {
	limiters := make(map[string]*webhookLimiter, len(targets))
	for _, t := range targets {
		limiters[t.URL] = &webhookLimiter{
			tokens:   10,
			max:      10,
			rate:     10.0 / 1.0,
			lastTick: time.Now(),
		}
	}
	return &WebhookDispatcher{
		targets: targets,
		client:  &http.Client{Timeout: 5 * time.Second},
		limiter: limiters,
	}
}

func (wd *WebhookDispatcher) Dispatch(e Event) {
	for _, t := range wd.targets {
		if !matchesFilter(e.Type, t.Events) {
			continue
		}
		if !wd.rateLimitAllow(t.URL) {
			slog.Warn("webhook rate limited", "url", t.URL, "event", e.Type)
			continue
		}
		go wd.deliver(t, e)
	}
}

func (wd *WebhookDispatcher) rateLimitAllow(url string) bool {
	wd.mu.Lock()
	defer wd.mu.Unlock()
	rl := wd.limiter[url]
	if rl == nil {
		return true
	}
	now := time.Now()
	elapsed := now.Sub(rl.lastTick).Seconds()
	rl.lastTick = now
	rl.tokens += elapsed * rl.rate
	if rl.tokens > rl.max {
		rl.tokens = rl.max
	}
	if rl.tokens < 1 {
		return false
	}
	rl.tokens--
	return true
}

func (wd *WebhookDispatcher) deliver(t WebhookTarget, e Event) {
	body, err := json.Marshal(e)
	if err != nil {
		slog.Error("webhook marshal", "error", err)
		return
	}

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	req, err := http.NewRequest(http.MethodPost, t.URL, bytes.NewReader(body))
	if err != nil {
		slog.Error("webhook request", "url", t.URL, "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Fox-Event", e.Type)
	req.Header.Set("X-Fox-Timestamp", timestamp)

	if t.Secret != "" {
		sig := computeHMAC(t.Secret, timestamp, body)
		req.Header.Set("X-Fox-Signature", sig)
	}

	resp, err := wd.client.Do(req)
	if err != nil {
		slog.Warn("webhook delivery failed", "url", t.URL, "event", e.Type, "error", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		slog.Warn("webhook delivery error", "url", t.URL, "event", e.Type, "status", resp.StatusCode)
	}
}

func computeHMAC(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func matchesFilter(eventType string, filter []string) bool {
	if len(filter) == 0 {
		return true
	}
	for _, f := range filter {
		if f == eventType {
			return true
		}
	}
	return false
}
