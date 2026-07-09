package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/auren/auren-transfer-agent/internal/logger"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

const (
	// MiddlewareNameRequestLogger identifies the canonical request logging middleware.
	MiddlewareNameRequestLogger = "request_logger"

	// MiddlewareNameRecoverer identifies the canonical panic recovery middleware.
	MiddlewareNameRecoverer = "recoverer"

	// MiddlewareComponent is the component name attached to middleware-level logs.
	MiddlewareComponent = "http_middleware"

	// MiddlewareOperationPanicRecovery identifies panic recovery log events.
	MiddlewareOperationPanicRecovery = "panic_recovery"
)

// MiddlewareDefinition is the canonical registration unit for HTTP middleware.
//
// It intentionally contains only transport-level behavior. Business decisions,
// job routing and transfer execution must remain outside the server package.
type MiddlewareDefinition struct {
	Name    string
	Handler func(http.Handler) http.Handler
}

// MiddlewareInfo describes middleware metadata for diagnostics and tests.
type MiddlewareInfo struct {
	Name string
}

// MiddlewareOptions controls the canonical foundation middleware stack.
type MiddlewareOptions struct {
	Logger         zerolog.Logger
	RequestLogging bool
	RecoverPanics  bool
	Middlewares    []MiddlewareDefinition
}

// MiddlewareRegistry stores middleware definitions before they are attached to a router.
type MiddlewareRegistry struct {
	middlewares []MiddlewareDefinition
}

// NewMiddlewareRegistry creates an empty registry and optionally adds middlewares.
func NewMiddlewareRegistry(middlewares ...MiddlewareDefinition) (*MiddlewareRegistry, error) {
	registry := &MiddlewareRegistry{}
	for _, middleware := range middlewares {
		if err := registry.Add(middleware); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

// DefaultMiddlewareRegistry builds the standard foundation middleware registry.
func DefaultMiddlewareRegistry(options MiddlewareOptions) (*MiddlewareRegistry, error) {
	registry := &MiddlewareRegistry{}

	if options.RequestLogging {
		if err := registry.Add(MiddlewareDefinition{
			Name:    MiddlewareNameRequestLogger,
			Handler: logger.RequestLogger(logger.WithFields(options.Logger, logger.String(logger.FieldComponent, RouterComponent))),
		}); err != nil {
			return nil, err
		}
	}

	if options.RecoverPanics {
		if err := registry.Add(MiddlewareDefinition{
			Name:    MiddlewareNameRecoverer,
			Handler: Recoverer(options.Logger),
		}); err != nil {
			return nil, err
		}
	}

	for _, middleware := range options.Middlewares {
		if err := registry.Add(middleware); err != nil {
			return nil, err
		}
	}

	return registry, nil
}

// Add validates and stores a middleware definition.
func (registry *MiddlewareRegistry) Add(middleware MiddlewareDefinition) error {
	if registry == nil {
		return fmt.Errorf("middleware registry cannot be nil")
	}

	normalized, err := NormalizeMiddlewareDefinition(middleware)
	if err != nil {
		return err
	}

	for _, existing := range registry.middlewares {
		if existing.Name == normalized.Name {
			return fmt.Errorf("middleware %s is already registered", normalized.Name)
		}
	}

	registry.middlewares = append(registry.middlewares, normalized)
	return nil
}

// Register attaches every middleware in the registry to the provided router.
func (registry *MiddlewareRegistry) Register(router chi.Router) error {
	if registry == nil {
		return nil
	}
	return RegisterMiddlewares(router, registry.middlewares...)
}

// Definitions returns a defensive copy of stored middleware definitions.
func (registry *MiddlewareRegistry) Definitions() []MiddlewareDefinition {
	if registry == nil || len(registry.middlewares) == 0 {
		return nil
	}

	output := make([]MiddlewareDefinition, len(registry.middlewares))
	copy(output, registry.middlewares)
	return output
}

// Middlewares returns a defensive copy of raw middleware handlers.
func (registry *MiddlewareRegistry) Middlewares() []func(http.Handler) http.Handler {
	if registry == nil || len(registry.middlewares) == 0 {
		return nil
	}

	output := make([]func(http.Handler) http.Handler, 0, len(registry.middlewares))
	for _, middleware := range registry.middlewares {
		output = append(output, middleware.Handler)
	}
	return output
}

// Snapshot returns middleware metadata without handlers for diagnostics and tests.
func (registry *MiddlewareRegistry) Snapshot() []MiddlewareInfo {
	if registry == nil || len(registry.middlewares) == 0 {
		return nil
	}

	output := make([]MiddlewareInfo, 0, len(registry.middlewares))
	for _, middleware := range registry.middlewares {
		output = append(output, MiddlewareInfo{Name: middleware.Name})
	}
	return output
}

// Len returns the number of registered middleware definitions.
func (registry *MiddlewareRegistry) Len() int {
	if registry == nil {
		return 0
	}
	return len(registry.middlewares)
}

// RegisterMiddlewares validates and attaches a batch of middlewares to a router.
func RegisterMiddlewares(router chi.Router, middlewares ...MiddlewareDefinition) error {
	if router == nil {
		return fmt.Errorf("router cannot be nil")
	}

	for _, middleware := range middlewares {
		normalized, err := NormalizeMiddlewareDefinition(middleware)
		if err != nil {
			return err
		}
		router.Use(normalized.Handler)
	}
	return nil
}

// NormalizeMiddlewareDefinition validates and canonicalizes a middleware definition.
func NormalizeMiddlewareDefinition(middleware MiddlewareDefinition) (MiddlewareDefinition, error) {
	name := NormalizeMiddlewareName(middleware.Name)
	if name == "" {
		return MiddlewareDefinition{}, fmt.Errorf("middleware name cannot be empty")
	}
	if middleware.Handler == nil {
		return MiddlewareDefinition{}, fmt.Errorf("middleware handler cannot be nil")
	}
	return MiddlewareDefinition{Name: name, Handler: middleware.Handler}, nil
}

// NormalizeMiddlewareName returns the canonical middleware name.
func NormalizeMiddlewareName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// Recoverer returns the foundation panic recovery middleware.
func Recoverer(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if next == nil {
			next = http.NotFoundHandler()
		}

		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					activeLog := log
					if request != nil {
						activeLog = logger.FromContextOrDefault(request.Context(), log)
					}
					logger.WithFields(activeLog,
						logger.String(logger.FieldComponent, MiddlewareComponent),
						logger.String(logger.FieldOperation, MiddlewareOperationPanicRecovery),
					).Error().Str("panic", fmt.Sprint(recovered)).Msg("http request panic recovered")
					http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(writer, request)
		})
	}
}
