package server

import (
	"encoding/json"
	"net/http"

	"github.com/auren/auren-transfer-agent/internal/runtime"
)

const (
	// HealthRouteName is the canonical diagnostic route name for the health endpoint.
	HealthRouteName = "health"

	// HealthPath is the canonical HTTP path for liveness diagnostics.
	HealthPath = "/health"

	// HealthResponseStatus is the stable status value returned by the foundation health endpoint.
	HealthResponseStatus = "ok"

	// HealthContentType is the canonical response content type for foundation JSON endpoints.
	HealthContentType = "application/json; charset=utf-8"
)

// HealthResponse is the stable liveness payload returned by the foundation health endpoint.
//
// It intentionally contains only runtime and transport metadata. Job execution,
// queue health, storage reachability and transfer decisions remain outside this
// endpoint and are reserved for later worker, upload, communication and ready phases.
type HealthResponse struct {
	Status        string `json:"status"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	RuntimeStatus string `json:"runtime_status"`
	Router        string `json:"router"`
}

// NewHealthResponse creates the canonical foundation liveness payload.
func NewHealthResponse(info runtime.VersionInfo) HealthResponse {
	return HealthResponse{
		Status:        HealthResponseStatus,
		Name:          info.Name,
		Version:       info.Version,
		RuntimeStatus: info.Status,
		Router:        RouterKindName(),
	}
}

// HealthHandler returns the foundation liveness HTTP handler.
func HealthHandler(info runtime.VersionInfo) http.HandlerFunc {
	payload := NewHealthResponse(info)
	return func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", HealthContentType)
		writer.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(writer).Encode(payload)
	}
}

// HealthRoute returns the canonical health route definition.
func HealthRoute(info runtime.VersionInfo) RouteDefinition {
	return RouteDefinition{
		Name:    HealthRouteName,
		Method:  http.MethodGet,
		Pattern: HealthPath,
		Handler: HealthHandler(info),
	}
}

// HealthRoutes returns the foundation route set introduced by EPIC 3.4.
func HealthRoutes(info runtime.VersionInfo) []RouteDefinition {
	return []RouteDefinition{HealthRoute(info)}
}
