package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/BadrChoubai/logistique/internal/config"
	"golang.org/x/time/rate"
)

// rateLimitError is the JSON body returned on 429 responses.
type rateLimitError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// ipRouteLimiter stores a token-bucket limiter keyed by "ip:route".
type ipRouteLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	config   config.RateLimitConfig
}

func newIPRouteLimiter(cfg config.RateLimitConfig) *ipRouteLimiter {
	return &ipRouteLimiter{
		limiters: make(map[string]*rate.Limiter),
		config:   cfg,
	}
}

func (irl *ipRouteLimiter) get(ip, route string) *rate.Limiter {
	key := ip + ":" + route

	irl.mu.Lock()
	defer irl.mu.Unlock()

	if lim, ok := irl.limiters[key]; ok {
		return lim
	}

	lim := rate.NewLimiter(rate.Limit(irl.config.RequestsPerSecond), irl.config.Burst)
	irl.limiters[key] = lim
	return lim
}

// RateLimit returns a Middleware that enforces a token-bucket rate limit
// scoped per client IP and matched route pattern.
//
// When the limit is exceeded the handler writes HTTP 429 with a JSON body and
// does not forward the request downstream.
//
// Usage:
//
//	g.Use(middleware.RateLimit(logistique.RateLimitConfig{
//	    RequestsPerSecond: 10,
//	    Burst:             20,
//	}))
func RateLimit(cfg config.RateLimitConfig) func(http.Handler) http.Handler {
	irl := newIPRouteLimiter(cfg)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			route := r.URL.Path

			if !irl.get(ip, route).Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(rateLimitError{
					Error:   "too_many_requests",
					Message: "rate limit exceeded, please slow down",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// clientIP extracts the originating IP from the request, preferring
// X-Forwarded-For (set by upstream proxies) and falling back to RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For may be a comma-separated list; the first entry is the
		// original client.
		if i := strings.IndexByte(xff, ','); i != -1 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// RemoteAddr is "host:port"; strip the port.
	addr := r.RemoteAddr
	if i := strings.LastIndexByte(addr, ':'); i != -1 {
		return addr[:i]
	}
	return addr
}
