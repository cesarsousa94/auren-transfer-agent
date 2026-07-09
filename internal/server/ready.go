package server

import (
	"encoding/json"
	"net/http"

	"github.com/cesarsousa94/auren-transfer-agent/internal/runtime"
)

const (
	// ReadyRouteName is the canonical diagnostic route name for the readiness endpoint.
	ReadyRouteName = "ready"

	// ReadyPath is the canonical HTTP path for readiness diagnostics.
	ReadyPath = "/ready"

	// ReadyResponseStatus is the stable status value returned by the foundation ready endpoint.
	ReadyResponseStatus = "ready"
)

// ReadyResponse is the stable readiness payload returned by the foundation ready endpoint.
//
// In the foundation line, readiness means the Agent configuration, router and middleware
// contracts were built successfully. Queue reachability, storage reachability, transfer
// worker capacity and Media Hub communication remain reserved for their own roadmap phases.
type ReadyResponse struct {
	Status        string `json:"status"`
	Ready         bool   `json:"ready"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	RuntimeStatus string `json:"runtime_status"`
	Router        string `json:"router"`
}

// NewReadyResponse creates the canonical foundation readiness payload.
func NewReadyResponse(info runtime.VersionInfo) ReadyResponse {
	return ReadyResponse{
		Status:        ReadyResponseStatus,
		Ready:         true,
		Name:          info.Name,
		Version:       info.Version,
		RuntimeStatus: info.Status,
		Router:        RouterKindName(),
	}
}

// ReadyHandler returns the foundation readiness HTTP handler.
func ReadyHandler(info runtime.VersionInfo) http.HandlerFunc {
	payload := NewReadyResponse(info)
	return func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", HealthContentType)
		writer.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(writer).Encode(payload)
	}
}

// ReadyRoute returns the canonical ready route definition.
func ReadyRoute(info runtime.VersionInfo) RouteDefinition {
	return RouteDefinition{
		Name:    ReadyRouteName,
		Method:  http.MethodGet,
		Pattern: ReadyPath,
		Handler: ReadyHandler(info),
	}
}

// ReadyRoutes returns the foundation route set introduced by EPIC 3.6.
func ReadyRoutes(info runtime.VersionInfo) []RouteDefinition {
	return []RouteDefinition{ReadyRoute(info)}
}
