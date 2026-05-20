# Logistique

A lightweight, composable HTTP reverse-proxy gateway for Go applications. Logistique makes it easy to route requests to multiple upstream services, apply middleware (like rate limiting), and manage cross-cutting concerns in a single place.

## Features

- 🚀 **Simple API Gateway** - Route requests to multiple upstream services with path-based prefixes
- 🔄 **Reverse Proxying** - Built on `httputil.ReverseProxy` for reliable request forwarding
- 🛡️ **Middleware Support** - Attach middleware like rate limiting with a clean, composable API
- 📝 **Path Rewriting** - Optionally rewrite request paths when forwarding to upstream services
- ⚡ **Fast & Lightweight** - Minimal dependencies, efficient routing
- 📊 **Built-in Rate Limiting** - Token bucket-based rate limiting middleware included
- 🔧 **Drop-in Integration** - Easily integrate into existing Go applications

## Installation

```bash
go get github.com/BadrChoubai/logistique
```

## Quick Start

Here's a minimal example that proxies requests to JSONPlaceholder:

```go
package main

import (
	"log"
	"net/http"
	"time"

	"github.com/BadrChoubai/logistique"
)

func main() {
	// Create a gateway with service configurations
	gw, err := logistique.New(
		logistique.Config{},
		logistique.ServiceConfig{
			Prefix:  "/api/posts/",
			Target:  "https://jsonplaceholder.typicode.com",
			Rewrite: "/posts",
		},
	)
	if err != nil {
		log.Fatalln(err)
	}

	// Start the server
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      gw.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Println("Gateway listening on :8080")
	log.Fatalln(srv.ListenAndServe())
}
```

Test it:

```bash
curl http://localhost:8080/api/posts/1
```

## Usage Examples

### Multiple Services with Rate Limiting

```go
import (
	"github.com/BadrChoubai/logistique"
	"github.com/BadrChoubai/logistique/middleware"
)

func main() {
	cfg := logistique.Config{
		RateLimit: logistique.RateLimitConfig{
			RequestsPerSecond: 10,
			Burst:             20,
		},
	}

	gw, err := logistique.New(
		cfg,
		logistique.ServiceConfig{
			Prefix: "/api/service-one/",
			Target: "http://localhost:8081",
		},
		logistique.ServiceConfig{
			Prefix: "/api/service-two/",
			Target: "http://localhost:8082",
		},
	)
	if err != nil {
		log.Fatalln(err)
	}

	// Apply rate limiting middleware
	gw.Use(middleware.RateLimit(cfg.RateLimit))

	srv := &http.Server{
		Addr:    ":8080",
		Handler: gw.Handler(),
	}

	log.Fatalln(srv.ListenAndServe())
}
```

### Custom Middleware

You can attach custom middleware to the gateway. Middleware is applied in registration order (outermost first):

```go
gw := logistique.New(/* ... */)

// Custom logging middleware
gw.Use(func(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Request: %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
})

gw.Use(middleware.RateLimit(cfg.RateLimit))
```

## API Documentation

### Gateway

The main type for creating and managing your HTTP gateway.

#### `New(config Config, services ...ServiceConfig) (*Gateway, error)`

Creates a new Gateway instance with the given configuration and upstream services.

```go
gw, err := logistique.New(
	logistique.Config{
		RateLimit: logistique.RateLimitConfig{
			RequestsPerSecond: 10,
			Burst:             20,
		},
	},
	logistique.ServiceConfig{
		Prefix: "/api/",
		Target: "http://backend:8080",
	},
)
```

**Parameters:**
- `config` - Gateway configuration (rate limiting settings)
- `services` - One or more upstream service configurations

**Returns:** A configured `*Gateway` or an error if service mounting fails

#### `Handler() http.Handler`

Returns the fully composed HTTP handler with all routes and middleware applied. Use this with `http.Server`:

```go
srv := &http.Server{
	Addr:    ":8080",
	Handler: gw.Handler(),
}
srv.ListenAndServe()
```

#### `Use(middleware ...Middleware)`

Attaches middleware to the gateway. Middleware is applied in registration order (outermost first):

```go
gw.Use(
	middleware.RateLimit(cfg.RateLimit),
	customLoggingMiddleware,
)
```

### ServiceConfig

Defines an upstream service to proxy requests to.

```go
type ServiceConfig struct {
	// Prefix is the path prefix the gateway will match against.
	// Requests matching this prefix will be routed to Target.
	// Example: "/api/users/"
	Prefix string

	// Target is the base URL of the upstream service.
	// Example: "http://users-service:8081"
	Target string

	// Rewrite is an optional path prefix to prepend after stripping Prefix.
	// If Prefix="/api/users/" and Rewrite="/v2", a request to
	// "/api/users/123" becomes "/v2/123" on the upstream service.
	// If empty, only the Prefix is stripped.
	Rewrite string
}
```

### Config

Gateway configuration structure.

```go
type Config struct {
	RateLimit RateLimitConfig
}
```

### RateLimitConfig

Token bucket configuration for rate limiting.

```go
type RateLimitConfig struct {
	// RequestsPerSecond is the steady-state refill rate of the token bucket.
	RequestsPerSecond float64

	// Burst is the maximum number of requests allowed in an instant.
	Burst int
}
```

### Middleware

Middleware is a function that wraps an `http.Handler`:

```go
type Middleware func(http.Handler) http.Handler
```

**Built-in Middleware:**
- `middleware.RateLimit(config RateLimitConfig)` - Token bucket-based rate limiting

## Project Structure

```
logistique/
├── logistique.go           # Main Gateway implementation
├── config.go               # Configuration types
├── middleware/
│   └── ratelimit.go        # Rate limiting middleware
├── internal/
│   └── router/             # Request routing
└── examples/
    ├── proxy_requests/     # Simple proxying example
    └── application_router/ # Integration with middleware example
```

## Development

### Prerequisites

- Go 1.26.2 or later
- golangci-lint (for linting)

### Building

```bash
make build
```

### Running Tests

```bash
make test
```

### Linting

```bash
make lint
```

### Continuous Integration

```bash
make ci    # Runs lint and test
```

## Examples

The repository includes working examples:

1. **Simple Proxy** (`examples/proxy_requests/`) - Proxies requests to JSONPlaceholder
2. **Application Router** (`examples/application_router/`) - Shows integration with rate limiting middleware

To run an example:

```bash
go run ./examples/proxy_requests/main.go
```

## Design Philosophy

Logistique follows these principles:

1. **Minimal Dependencies** - Only imports `golang.org/x/time` for rate limiting
2. **Composable** - Middleware can be easily added and combined
3. **Standard Library Friendly** - Built on `net/http` and `httputil.ReverseProxy`
4. **Production Ready** - Designed for real-world integration (based on proven patterns from the "resonance" project)

## License

License information can be found in the repository.

## Contributing

Contributions are welcome! Please feel free to open issues and pull requests.
