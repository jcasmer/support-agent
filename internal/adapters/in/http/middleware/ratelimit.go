package middleware

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// client holds the rate limiter and last seen time for a single IP
type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter manages per-IP token bucket limiters
type RateLimiter struct {
	clients  map[string]*client
	mu       sync.Mutex
	rate     rate.Limit
	burst    int
	stopOnce sync.Once
	stop     chan struct{}
}

// NewRateLimiter creates a new RateLimiter.
// r is the number of requests allowed per second.
// burst is the maximum number of requests allowed in a burst.
func NewRateLimiter(r rate.Limit, burst int) *RateLimiter {
	rl := &RateLimiter{
		clients: make(map[string]*client),
		rate:    r,
		burst:   burst,
		stop:    make(chan struct{}),
	}

	// Start background cleanup goroutine
	go rl.cleanup()

	return rl
}

// Stop shuts down the background cleanup goroutine
func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() {
		close(rl.stop)
	})
}

// Limit returns a middleware that enforces the rate limit per client IP
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)

		limiter := rl.getLimiter(ip)

		if !limiter.Allow() {
			slog.Warn("rate limit exceeded",
				"ip", ip,
				"path", r.URL.Path,
			)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{
				"code":    "rate_limit_exceeded",
				"message": "too many requests, please try again later",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getLimiter returns the rate limiter for the given IP,
// creating one if it does not exist
func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	c, exists := rl.clients[ip]
	if !exists {
		c = &client{
			limiter: rate.NewLimiter(rl.rate, rl.burst),
		}
		rl.clients[ip] = c
	}

	c.lastSeen = time.Now()
	return c.limiter
}

// cleanup removes clients that have not been seen for more than 3 minutes.
// Runs every minute in the background to prevent memory leaks.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			for ip, c := range rl.clients {
				if time.Since(c.lastSeen) > 3*time.Minute {
					delete(rl.clients, ip)
				}
			}
			rl.mu.Unlock()
		case <-rl.stop:
			return
		}
	}
}

// extractIP extracts the real client IP from the request.
// It checks X-Forwarded-For first (for requests behind a proxy),
// then falls back to RemoteAddr.
func extractIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return forwarded
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}