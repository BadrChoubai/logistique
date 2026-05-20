package logistique

type Config struct {
	RateLimit RateLimitConfig
}

// RateLimitConfig holds the token bucket parameters.
type RateLimitConfig struct {
	// RequestsPerSecond is the steady-state refill rate of the token bucket.
	RequestsPerSecond float64
	// Burst is the maximum number of requests allowed in an instant.
	Burst int
}
