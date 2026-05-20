package logistique

type Config struct {
	RateLimit RateLimitConfig
}

type RateLimitConfig struct {
	RequestsPerSecond float64
	Burst             int
}
