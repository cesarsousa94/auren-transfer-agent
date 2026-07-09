package server

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
)

const (
	// RouteNameSeparator separates method and pattern in generated route names.
	RouteNameSeparator = " "
)

// RouteDefinition is the canonical registration unit for the Agent HTTP router.
//
// It intentionally contains only transport-level data. Business rules, transfer
// decisions and Media Hub workflows must stay outside the Agent router layer.
type RouteDefinition struct {
	Name    string
	Method  string
	Pattern string
	Handler http.HandlerFunc
}

// RouteInfo describes a registered foundation route for tests and diagnostics.
type RouteInfo struct {
	Name    string
	Method  string
	Pattern string
}

// RouteRegistry stores route definitions before they are attached to a router.
type RouteRegistry struct {
	routes []RouteDefinition
}

// NewRouteRegistry creates an empty registry and optionally adds routes.
func NewRouteRegistry(routes ...RouteDefinition) (*RouteRegistry, error) {
	registry := &RouteRegistry{}
	for _, route := range routes {
		if err := registry.Add(route); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

// Add validates and stores a route definition.
func (registry *RouteRegistry) Add(route RouteDefinition) error {
	if registry == nil {
		return fmt.Errorf("route registry cannot be nil")
	}

	normalized, err := NormalizeRouteDefinition(route)
	if err != nil {
		return err
	}

	for _, existing := range registry.routes {
		if existing.Method == normalized.Method && existing.Pattern == normalized.Pattern {
			return fmt.Errorf("route %s %s is already registered", normalized.Method, normalized.Pattern)
		}
	}

	registry.routes = append(registry.routes, normalized)
	return nil
}

// Register attaches every route in the registry to the provided router.
func (registry *RouteRegistry) Register(router chi.Router) error {
	if registry == nil {
		return nil
	}
	return RegisterRoutes(router, registry.routes...)
}

// Routes returns a defensive copy of stored route definitions.
func (registry *RouteRegistry) Routes() []RouteDefinition {
	if registry == nil || len(registry.routes) == 0 {
		return nil
	}

	output := make([]RouteDefinition, len(registry.routes))
	copy(output, registry.routes)
	return output
}

// Snapshot returns route metadata without handlers for diagnostics and tests.
func (registry *RouteRegistry) Snapshot() []RouteInfo {
	if registry == nil || len(registry.routes) == 0 {
		return nil
	}

	output := make([]RouteInfo, 0, len(registry.routes))
	for _, route := range registry.routes {
		output = append(output, route.Info())
	}
	return output
}

// Len returns the number of registered route definitions.
func (registry *RouteRegistry) Len() int {
	if registry == nil {
		return 0
	}
	return len(registry.routes)
}

// RegisterRoutes validates and attaches a batch of route definitions.
func RegisterRoutes(router chi.Router, routes ...RouteDefinition) error {
	if router == nil {
		return fmt.Errorf("router cannot be nil")
	}

	for _, route := range routes {
		if err := RegisterRouteDefinition(router, route); err != nil {
			return err
		}
	}
	return nil
}

// RegisterRouteDefinition validates and attaches a single route definition.
func RegisterRouteDefinition(router chi.Router, route RouteDefinition) error {
	if router == nil {
		return fmt.Errorf("router cannot be nil")
	}

	normalized, err := NormalizeRouteDefinition(route)
	if err != nil {
		return err
	}

	router.MethodFunc(normalized.Method, normalized.Pattern, normalized.Handler)
	return nil
}

// NormalizeRouteDefinition validates and canonicalizes a route definition.
func NormalizeRouteDefinition(route RouteDefinition) (RouteDefinition, error) {
	method := NormalizeMethod(route.Method)
	if method == "" {
		return RouteDefinition{}, fmt.Errorf("route method cannot be empty")
	}
	if !IsSupportedHTTPMethod(method) {
		return RouteDefinition{}, fmt.Errorf("unsupported route method %q", route.Method)
	}

	pattern := NormalizePattern(route.Pattern)
	if pattern == "" {
		return RouteDefinition{}, fmt.Errorf("route pattern cannot be empty")
	}
	if !strings.HasPrefix(pattern, "/") {
		return RouteDefinition{}, fmt.Errorf("route pattern %q must start with /", route.Pattern)
	}
	if route.Handler == nil {
		return RouteDefinition{}, fmt.Errorf("route handler cannot be nil")
	}

	name := strings.TrimSpace(route.Name)
	if name == "" {
		name = RouteName(method, pattern)
	}

	return RouteDefinition{Name: name, Method: method, Pattern: pattern, Handler: route.Handler}, nil
}

// Info returns diagnostic metadata for a route definition.
func (route RouteDefinition) Info() RouteInfo {
	return RouteInfo{Name: route.Name, Method: route.Method, Pattern: route.Pattern}
}

// RouteName returns a deterministic route name from method and pattern.
func RouteName(method string, pattern string) string {
	return NormalizeMethod(method) + RouteNameSeparator + NormalizePattern(pattern)
}

// NormalizeMethod returns the canonical uppercase HTTP method.
func NormalizeMethod(method string) string {
	return strings.ToUpper(strings.TrimSpace(method))
}

// NormalizePattern returns the canonical route pattern.
func NormalizePattern(pattern string) string {
	trimmed := strings.TrimSpace(pattern)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "/") {
		return "/" + trimmed
	}
	return trimmed
}

// IsSupportedHTTPMethod reports whether method is part of the foundation router contract.
func IsSupportedHTTPMethod(method string) bool {
	_, ok := supportedHTTPMethods()[NormalizeMethod(method)]
	return ok
}

// SupportedHTTPMethods returns a sorted defensive copy of methods accepted by the router contract.
func SupportedHTTPMethods() []string {
	methods := make([]string, 0, len(supportedHTTPMethods()))
	for method := range supportedHTTPMethods() {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	return methods
}

func supportedHTTPMethods() map[string]struct{} {
	return map[string]struct{}{
		http.MethodConnect: {},
		http.MethodDelete:  {},
		http.MethodGet:     {},
		http.MethodHead:    {},
		http.MethodOptions: {},
		http.MethodPatch:   {},
		http.MethodPost:    {},
		http.MethodPut:     {},
		http.MethodTrace:   {},
	}
}
