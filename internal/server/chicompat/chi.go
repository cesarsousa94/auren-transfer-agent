// Package chi is a compact local compatibility layer for the subset of
// github.com/go-chi/chi/v5 used by the Auren Transfer Agent foundation.
//
// The official source code imports github.com/go-chi/chi/v5 through go.mod.
// The local replace keeps official ZIPs self-contained and offline-compilable
// until production packaging switches to the upstream module.
package chi

import (
	"net/http"
	"strings"
)

// Middlewares is the standard Chi middleware chain type.
type Middlewares []func(http.Handler) http.Handler

// Router is the subset of the Chi router contract used by the Agent roadmap.
type Router interface {
	http.Handler

	Use(middlewares ...func(http.Handler) http.Handler)
	With(middlewares ...func(http.Handler) http.Handler) Router
	Group(fn func(r Router)) Router
	Route(pattern string, fn func(r Router)) Router
	Mount(pattern string, handler http.Handler)
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler http.HandlerFunc)
	Method(method string, pattern string, handler http.Handler)
	MethodFunc(method string, pattern string, handler http.HandlerFunc)

	Connect(pattern string, handler http.HandlerFunc)
	Delete(pattern string, handler http.HandlerFunc)
	Get(pattern string, handler http.HandlerFunc)
	Head(pattern string, handler http.HandlerFunc)
	Options(pattern string, handler http.HandlerFunc)
	Patch(pattern string, handler http.HandlerFunc)
	Post(pattern string, handler http.HandlerFunc)
	Put(pattern string, handler http.HandlerFunc)
	Trace(pattern string, handler http.HandlerFunc)

	NotFound(handler http.HandlerFunc)
	MethodNotAllowed(handler http.HandlerFunc)
}

type route struct {
	method  string
	pattern string
	handler http.Handler
}

// Mux is a small method-aware router compatible with the subset of Chi used by
// the Agent foundation tests. It is not intended to replace the upstream router
// in production distributions.
type Mux struct {
	prefix           string
	routes           []route
	middlewares      Middlewares
	notFound         http.Handler
	methodNotAllowed http.Handler
}

// NewRouter creates an empty compatibility router.
func NewRouter() *Mux {
	return &Mux{
		notFound:         http.NotFoundHandler(),
		methodNotAllowed: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusMethodNotAllowed) }),
	}
}

// Use appends middlewares to the current router.
func (mux *Mux) Use(middlewares ...func(http.Handler) http.Handler) {
	mux.middlewares = append(mux.middlewares, middlewares...)
}

// With returns a shallow router clone with additional middlewares.
func (mux *Mux) With(middlewares ...func(http.Handler) http.Handler) Router {
	child := mux.clone()
	child.middlewares = append(child.middlewares, middlewares...)
	return child
}

// Group creates a grouped router that shares the current prefix and middlewares.
func (mux *Mux) Group(fn func(r Router)) Router {
	child := mux.clone()
	if fn != nil {
		fn(child)
		mux.routes = append(mux.routes, child.routes...)
	}
	return child
}

// Route creates a grouped router below pattern.
func (mux *Mux) Route(pattern string, fn func(r Router)) Router {
	child := mux.clone()
	child.prefix = joinPattern(child.prefix, pattern)
	child.routes = nil
	if fn != nil {
		fn(child)
		mux.routes = append(mux.routes, child.routes...)
	}
	return child
}

// Mount registers a handler for every request below pattern.
func (mux *Mux) Mount(pattern string, handler http.Handler) {
	if handler == nil {
		handler = http.NotFoundHandler()
	}
	mux.routes = append(mux.routes, route{method: "", pattern: joinPattern(mux.prefix, pattern), handler: handler})
}

// Handle registers a handler for every method at pattern.
func (mux *Mux) Handle(pattern string, handler http.Handler) {
	mux.Method("", pattern, handler)
}

// HandleFunc registers a function handler for every method at pattern.
func (mux *Mux) HandleFunc(pattern string, handler http.HandlerFunc) {
	mux.Handle(pattern, handler)
}

// Method registers a method-specific handler.
func (mux *Mux) Method(method string, pattern string, handler http.Handler) {
	if handler == nil {
		handler = http.NotFoundHandler()
	}
	mux.routes = append(mux.routes, route{method: strings.ToUpper(strings.TrimSpace(method)), pattern: joinPattern(mux.prefix, pattern), handler: handler})
}

// MethodFunc registers a method-specific function handler.
func (mux *Mux) MethodFunc(method string, pattern string, handler http.HandlerFunc) {
	mux.Method(method, pattern, handler)
}

func (mux *Mux) Connect(pattern string, handler http.HandlerFunc) {
	mux.MethodFunc(http.MethodConnect, pattern, handler)
}
func (mux *Mux) Delete(pattern string, handler http.HandlerFunc) {
	mux.MethodFunc(http.MethodDelete, pattern, handler)
}
func (mux *Mux) Get(pattern string, handler http.HandlerFunc) {
	mux.MethodFunc(http.MethodGet, pattern, handler)
}
func (mux *Mux) Head(pattern string, handler http.HandlerFunc) {
	mux.MethodFunc(http.MethodHead, pattern, handler)
}
func (mux *Mux) Options(pattern string, handler http.HandlerFunc) {
	mux.MethodFunc(http.MethodOptions, pattern, handler)
}
func (mux *Mux) Patch(pattern string, handler http.HandlerFunc) {
	mux.MethodFunc(http.MethodPatch, pattern, handler)
}
func (mux *Mux) Post(pattern string, handler http.HandlerFunc) {
	mux.MethodFunc(http.MethodPost, pattern, handler)
}
func (mux *Mux) Put(pattern string, handler http.HandlerFunc) {
	mux.MethodFunc(http.MethodPut, pattern, handler)
}
func (mux *Mux) Trace(pattern string, handler http.HandlerFunc) {
	mux.MethodFunc(http.MethodTrace, pattern, handler)
}

// NotFound replaces the fallback 404 handler.
func (mux *Mux) NotFound(handler http.HandlerFunc) {
	if handler != nil {
		mux.notFound = handler
	}
}

// MethodNotAllowed replaces the fallback 405 handler.
func (mux *Mux) MethodNotAllowed(handler http.HandlerFunc) {
	if handler != nil {
		mux.methodNotAllowed = handler
	}
}

// ServeHTTP dispatches the request through the registered route table.
func (mux *Mux) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	handler := http.Handler(http.HandlerFunc(mux.dispatch))
	for index := len(mux.middlewares) - 1; index >= 0; index-- {
		if mux.middlewares[index] != nil {
			handler = mux.middlewares[index](handler)
		}
	}
	handler.ServeHTTP(writer, request)
}

func (mux *Mux) dispatch(writer http.ResponseWriter, request *http.Request) {
	if request == nil {
		mux.notFound.ServeHTTP(writer, request)
		return
	}

	methodNotAllowed := false
	for _, candidate := range mux.routes {
		if !pathMatches(candidate.pattern, request.URL.Path) {
			continue
		}
		if candidate.method == "" || candidate.method == request.Method {
			candidate.handler.ServeHTTP(writer, request)
			return
		}
		methodNotAllowed = true
	}

	if methodNotAllowed {
		mux.methodNotAllowed.ServeHTTP(writer, request)
		return
	}
	mux.notFound.ServeHTTP(writer, request)
}

func (mux *Mux) clone() *Mux {
	child := *mux
	child.middlewares = append(Middlewares{}, mux.middlewares...)
	child.routes = append([]route{}, mux.routes...)
	return &child
}

func joinPattern(prefix string, pattern string) string {
	cleanPrefix := strings.TrimRight(strings.TrimSpace(prefix), "/")
	cleanPattern := strings.TrimSpace(pattern)
	if cleanPattern == "" {
		cleanPattern = "/"
	}
	if !strings.HasPrefix(cleanPattern, "/") {
		cleanPattern = "/" + cleanPattern
	}
	if cleanPrefix == "" || cleanPrefix == "/" {
		return cleanPattern
	}
	if cleanPattern == "/" {
		return cleanPrefix
	}
	return cleanPrefix + cleanPattern
}

func pathMatches(pattern string, path string) bool {
	if pattern == "" {
		pattern = "/"
	}
	if pattern == path {
		return true
	}
	if strings.HasSuffix(pattern, "/*") {
		return strings.HasPrefix(path, strings.TrimSuffix(pattern, "*"))
	}
	return false
}
