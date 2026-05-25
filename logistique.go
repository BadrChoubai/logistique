package logistique

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/BadrChoubai/logistique/internal/config"
	"github.com/BadrChoubai/logistique/internal/router"
)

// ServiceConfig defines an upstream service to proxy to.
type ServiceConfig struct {
	// Prefix is the path prefix the gateway will match against.
	// e.g. "/api/journal/"
	Prefix string
	// Target is the base URL of the upstream service.
	// e.g. "http://journal:8081"
	Target string

	// If Rewrite is empty, the stripped path is forwarded as-is:
	//
	//	Prefix:  "/api/journal/"
	//	Rewrite: ""
	//	Request: /api/journal/entries/42
	//	Upstream: /entries/42
	//
	// If Rewrite is set, the stripped path is mounted under Rewrite:
	//
	//	Prefix:  "/api/journal/"
	//	Rewrite: "/v1"
	//	Request: /api/journal/entries/42
	//	Upstream: /v1/entries/42
	//
	// This is useful when the public gateway route differs from the upstream API
	// structure.
	Rewrite string
}

type Middleware func(http.Handler) http.Handler

// Gateway is a reverse-proxying HTTP gateway.
// Construct one with New and pass its Handler to an http.Server.
type Gateway struct {
	rtr         *router.Router
	middlewares []Middleware
	services    []ServiceConfig
}

func New(config config.Config, services ...ServiceConfig) (*Gateway, error) {
	g := &Gateway{rtr: router.NewRouter()}

	for _, svc := range services {
		if err := g.mount(svc); err != nil {
			return nil, fmt.Errorf("logistique: mounting %q: %w", svc.Prefix, err)
		}
	}

	g.logRoutes(log.Default())
	return g, nil
}

func (g *Gateway) Use(m ...Middleware) {
	g.middlewares = append(g.middlewares, m...)
}

// Handler returns the gateway's http.Handler for use with an http.Server.
func (g *Gateway) Handler() http.Handler {
	h := g.rtr.Handler()
	for i := len(g.middlewares) - 1; i >= 0; i-- {
		h = g.middlewares[i](h)
	}

	return h
}

func (g *Gateway) logRoutes(logger *log.Logger) {
	for _, svc := range g.services {
		logger.Printf("route: %s -> %s", svc.Prefix, svc.Target)
	}
}

func (g *Gateway) mount(svc ServiceConfig) error {
	target, err := url.Parse(svc.Target)
	if err != nil {
		return err
	}
	g.rtr.AddRoute("", svc.Prefix, newProxy(target, svc))
	g.services = append(g.services, svc)
	return nil
}

func newProxy(target *url.URL, svc ServiceConfig) http.Handler {
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.Host = target.Host

			stripped := strings.TrimPrefix(r.In.URL.Path, svc.Prefix)
			base := strings.TrimRight(svc.Rewrite, "/")
			path := strings.TrimLeft(stripped, "/")

			r.Out.URL.Path = base + "/" + path

			r.Out.Header.Set("Accept", "application/json")
			log.Printf("rewrite: %s -> %s%s", r.In.URL.Path, r.Out.Host, r.Out.URL.Path)
		},
	}
}
