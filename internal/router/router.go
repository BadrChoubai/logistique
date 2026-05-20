package router

import "net/http"

type Router struct {
	mux *http.ServeMux
}

func NewRouter() *Router {
	return &Router{
		mux: http.NewServeMux(),
	}
}

func (r *Router) AddRoute(method, path string, handler http.Handler) {
	pattern := path
	if method != "" {
		pattern = method + " " + path
	}
	r.mux.Handle(pattern, handler)
}

func (r *Router) Handler() http.Handler {
	return r.mux
}
