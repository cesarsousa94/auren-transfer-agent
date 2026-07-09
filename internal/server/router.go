// Package server contains the foundation HTTP routing contract.
package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

const (
	// RouterKind identifies the router implementation selected by the roadmap.
	RouterKind = "chi"

	// RouterComponent is the component name attached to router-level logs.
	RouterComponent = "http_router"
)

// RouterOptions controls the foundation router factory.
type RouterOptions struct {
	Logger         zerolog.Logger
	RequestLogging bool
	RecoverPanics  bool
	Middlewares    []MiddlewareDefinition
	Routes         []RouteDefinition
}

// NewRouter creates the canonical Chi router for the Agent HTTP stack.
//
// v0.1.24 wires route, middleware, health, version, ready, identity and worker API
// contracts while concrete server lifecycle endpoints remain reserved for later phases.
func NewRouter(options RouterOptions) chi.Router {
	router, _ := BuildRouter(options)
	return router
}

// BuildRouter creates the canonical Chi router and registers supplied routes.
func BuildRouter(options RouterOptions) (chi.Router, error) {
	router := chi.NewRouter()
	middlewares, err := DefaultMiddlewareRegistry(MiddlewareOptions{
		Logger:         options.Logger,
		RequestLogging: options.RequestLogging,
		RecoverPanics:  options.RecoverPanics,
		Middlewares:    options.Middlewares,
	})
	if err != nil {
		return nil, err
	}
	if err := middlewares.Register(router); err != nil {
		return nil, err
	}

	if err := RegisterRoutes(router, options.Routes...); err != nil {
		return nil, err
	}

	return router, nil
}

// RegisterRoute attaches a handler to the router using the Chi method API.
//
// It preserves the defensive v0.1.11 behavior by ignoring nil inputs. New code
// should prefer RegisterRouteDefinition or RegisterRoutes when it needs errors.
func RegisterRoute(router chi.Router, route RouteInfo, handler http.HandlerFunc) {
	if router == nil || handler == nil {
		return
	}

	_ = RegisterRouteDefinition(router, RouteDefinition{Name: route.Name, Method: route.Method, Pattern: route.Pattern, Handler: handler})
}

// RouterKindName returns the configured router family.
func RouterKindName() string {
	return RouterKind
}
