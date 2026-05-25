package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BadrChoubai/logistique/internal/config"
	"github.com/BadrChoubai/logistique/middleware"
)

// okHandler is a trivial downstream that always returns 200.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func newRequest(method, path, remoteAddr, xff string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	return req
}

func TestRateLimit_AllowsWithinBurst(t *testing.T) {
	cfg := config.RateLimitConfig{RequestsPerSecond: 1, Burst: 5}
	handler := middleware.RateLimit(cfg)(okHandler)

	for i := range 5 {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, newRequest("GET", "/api/resource", "10.0.0.1:1234", ""))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: want 200, got %d", i+1, rec.Code)
		}
	}
}

func TestRateLimit_BlocksAfterBurstExhausted(t *testing.T) {
	cfg := config.RateLimitConfig{RequestsPerSecond: 1, Burst: 3}
	handler := middleware.RateLimit(cfg)(okHandler)

	ip := "10.0.0.2:9999"
	path := "/api/resource"

	// Drain the bucket.
	for range 3 {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, newRequest("GET", path, ip, ""))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected burst requests to succeed")
		}
	}

	// Next request must be blocked.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newRequest("GET", path, ip, ""))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429 after burst exhausted, got %d", rec.Code)
	}
}

func TestRateLimit_429ResponseIsJSON(t *testing.T) {
	cfg := config.RateLimitConfig{RequestsPerSecond: 0.001, Burst: 1}
	handler := middleware.RateLimit(cfg)(okHandler)

	ip := "10.0.0.3:1111"
	path := "/api/resource"

	// First request drains the single-token bucket.
	handler.ServeHTTP(httptest.NewRecorder(), newRequest("GET", path, ip, ""))

	// Second request should 429 with a parseable JSON body.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newRequest("GET", path, ip, ""))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("want Content-Type application/json, got %q", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if body["error"] == "" {
		t.Error("JSON body missing 'error' field")
	}
	if body["message"] == "" {
		t.Error("JSON body missing 'message' field")
	}
}

// TestRateLimit_IsolationByRoute verifies that two different routes for the
// same IP maintain independent buckets.
func TestRateLimit_IsolationByRoute(t *testing.T) {
	cfg := config.RateLimitConfig{RequestsPerSecond: 1, Burst: 1}
	handler := middleware.RateLimit(cfg)(okHandler)

	ip := "10.0.0.4:2222"

	// Drain /api/a for this IP.
	handler.ServeHTTP(httptest.NewRecorder(), newRequest("GET", "/api/a", ip, ""))

	// /api/b for the same IP should still have a full bucket.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newRequest("GET", "/api/b", ip, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("different route should have its own bucket; got %d", rec.Code)
	}
}

// TestRateLimit_IsolationByIP verifies that two different IPs on the same
// route maintain independent buckets.
func TestRateLimit_IsolationByIP(t *testing.T) {
	cfg := config.RateLimitConfig{RequestsPerSecond: 1, Burst: 1}
	handler := middleware.RateLimit(cfg)(okHandler)

	path := "/api/resource"

	// Drain the bucket for ip1.
	handler.ServeHTTP(httptest.NewRecorder(), newRequest("GET", path, "10.0.0.5:1000", ""))

	// ip2 on the same route should have a full bucket.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newRequest("GET", path, "10.0.0.6:1000", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("different IP should have its own bucket; got %d", rec.Code)
	}
}

// TestRateLimit_XForwardedFor checks that the middleware correctly uses
// the first address in an X-Forwarded-For header for keying.
func TestRateLimit_XForwardedFor(t *testing.T) {
	cfg := config.RateLimitConfig{RequestsPerSecond: 1, Burst: 1}
	handler := middleware.RateLimit(cfg)(okHandler)

	path := "/api/resource"
	xff := "203.0.113.5, 10.0.0.1"

	// Drain bucket for the XFF client.
	handler.ServeHTTP(httptest.NewRecorder(), newRequest("GET", path, "10.0.0.1:80", xff))

	// Same XFF from a different proxy RemoteAddr — same bucket, should block.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newRequest("GET", path, "10.0.0.2:80", xff))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("same XFF origin should share a bucket; got %d", rec.Code)
	}
}
