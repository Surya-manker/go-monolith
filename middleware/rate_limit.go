package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

type tokenBucket struct {
	tokens   float64
	lastSeen time.Time
}

// RateLimiter is a per-IP token bucket rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*tokenBucket
	rate     float64 // tokens added per second
	capacity float64
}

func NewRateLimiter(rps, capacity float64) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*tokenBucket),
		rate:     rps,
		capacity: capacity,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	b, ok := rl.buckets[ip]
	if !ok {
		b = &tokenBucket{tokens: rl.capacity, lastSeen: now}
		rl.buckets[ip] = b
	}
	b.tokens = min(rl.capacity, b.tokens+now.Sub(b.lastSeen).Seconds()*rl.rate)
	b.lastSeen = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !rl.Allow(ip) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// cleanup removes stale buckets every 10 minutes.
func (rl *RateLimiter) cleanup() {
	for range time.NewTicker(10 * time.Minute).C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-15 * time.Minute)
		for ip, b := range rl.buckets {
			if b.lastSeen.Before(cutoff) {
				delete(rl.buckets, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.Split(xff, ",")[0]
	}
	return r.RemoteAddr
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
