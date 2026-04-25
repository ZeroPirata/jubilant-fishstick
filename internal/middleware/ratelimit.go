package middleware

import (
	"hackton-treino/internal/util"
	"net/http"
	"sync"
	"time"
)

type ipEntry struct {
	count   int
	resetAt time.Time
}

type RateLimiter struct {
	mu        sync.Mutex
	entries   map[string]*ipEntry
	max       int
	window    time.Duration
	onBlocked func(r *http.Request)
}

func NewRateLimiter(max int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		entries: make(map[string]*ipEntry),
		max:     max,
		window:  window,
	}
	go rl.cleanup()
	return rl
}

// OnBlocked registers a callback invoked when a request is rate-limited.
func (rl *RateLimiter) OnBlocked(fn func(r *http.Request)) *RateLimiter {
	rl.onBlocked = fn
	return rl
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, entry := range rl.entries {
			if now.After(entry.resetAt) {
				delete(rl.entries, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, ok := rl.entries[ip]
	if !ok || now.After(entry.resetAt) {
		rl.entries[ip] = &ipEntry{count: 1, resetAt: now.Add(rl.window)}
		return true
	}
	if entry.count >= rl.max {
		return false
	}
	entry.count++
	return true
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.allow(util.ClientIP(r)) {
			if rl.onBlocked != nil {
				rl.onBlocked(r)
			}
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
