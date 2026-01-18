package middleware

import (
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter per IP
type RateLimiter struct {
	mu       sync.RWMutex
	clients  map[string]*client
	rate     int           // requests per window
	window   time.Duration // time window
	cleanup  time.Duration // cleanup interval for stale entries
	stopChan chan struct{}
}

type client struct {
	tokens    int
	lastReset time.Time
}

// NewRateLimiter creates a new rate limiter
// rate: max requests per window
// window: time window (e.g., 1 minute)
func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		clients:  make(map[string]*client),
		rate:     rate,
		window:   window,
		cleanup:  5 * time.Minute,
		stopChan: make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	log.Printf("[RATELIMIT] Rate limiter initialized: %d requests per %s", rate, window)
	return rl
}

// Allow checks if a request from the given IP is allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	c, exists := rl.clients[ip]

	if !exists {
		// New client, create entry with full tokens minus one for this request
		rl.clients[ip] = &client{
			tokens:    rl.rate - 1,
			lastReset: now,
		}
		return true
	}

	// Check if window has passed, reset tokens
	if now.Sub(c.lastReset) >= rl.window {
		c.tokens = rl.rate - 1
		c.lastReset = now
		return true
	}

	// Check if tokens available
	if c.tokens > 0 {
		c.tokens--
		return true
	}

	return false
}

// RemainingTokens returns the number of remaining requests for an IP
func (rl *RateLimiter) RemainingTokens(ip string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	c, exists := rl.clients[ip]
	if !exists {
		return rl.rate
	}

	// Check if window has passed
	if time.Since(c.lastReset) >= rl.window {
		return rl.rate
	}

	return c.tokens
}

// cleanupLoop periodically removes stale entries
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup_stale()
		case <-rl.stopChan:
			return
		}
	}
}

func (rl *RateLimiter) cleanup_stale() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	threshold := time.Now().Add(-2 * rl.window)
	removed := 0

	for ip, c := range rl.clients {
		if c.lastReset.Before(threshold) {
			delete(rl.clients, ip)
			removed++
		}
	}

	if removed > 0 {
		log.Printf("[RATELIMIT] Cleaned up %d stale entries", removed)
	}
}

// Stop stops the cleanup goroutine
func (rl *RateLimiter) Stop() {
	close(rl.stopChan)
}

// Middleware returns an HTTP middleware that applies rate limiting
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)

		if !rl.Allow(ip) {
			log.Printf("[RATELIMIT] Rate limit exceeded for IP: %s", ip)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error": "rate limit exceeded", "message": "too many requests, please try again later"}`))
			return
		}

		// Add rate limit headers
		w.Header().Set("X-RateLimit-Limit", string(rune(rl.rate)))
		w.Header().Set("X-RateLimit-Remaining", string(rune(rl.RemainingTokens(ip))))

		next.ServeHTTP(w, r)
	})
}

// getClientIP extracts the real client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (first IP is the client)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" {
				return ip
			}
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
