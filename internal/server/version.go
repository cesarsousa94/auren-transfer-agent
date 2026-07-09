package server

import (
	"encoding/json"
	"net/http"

	agentidentity "github.com/auren/auren-transfer-agent/internal/identity"
	"github.com/auren/auren-transfer-agent/internal/runtime"
)

const (
	// VersionRouteName is the canonical diagnostic route name for the version endpoint.
	VersionRouteName = "version"

	// VersionPath is the canonical HTTP path for runtime version diagnostics.
	VersionPath = "/version"
)

// VersionResponse is the stable runtime metadata payload returned by the foundation version endpoint.
//
// It intentionally exposes only immutable runtime metadata and router identity. Operational readiness,
// queue state, storage reachability and transfer engine state are exposed through separate roadmap contracts.
type VersionResponse struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	RuntimeStatus string `json:"runtime_status"`
	Router        string `json:"router"`
}

// NewVersionResponse creates the canonical foundation version payload.
func NewVersionResponse(info runtime.VersionInfo) VersionResponse {
	return VersionResponse{
		Name:          info.Name,
		Version:       info.Version,
		RuntimeStatus: info.Status,
		Router:        RouterKindName(),
	}
}

// VersionHandler returns the foundation runtime version HTTP handler.
func VersionHandler(info runtime.VersionInfo) http.HandlerFunc {
	payload := NewVersionResponse(info)
	return func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", HealthContentType)
		writer.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(writer).Encode(payload)
	}
}

// VersionRoute returns the canonical version route definition.
func VersionRoute(info runtime.VersionInfo) RouteDefinition {
	return RouteDefinition{
		Name:    VersionRouteName,
		Method:  http.MethodGet,
		Pattern: VersionPath,
		Handler: VersionHandler(info),
	}
}

// VersionRoutes returns the foundation route set introduced by EPIC 3.5.
func VersionRoutes(info runtime.VersionInfo) []RouteDefinition {
	return []RouteDefinition{VersionRoute(info)}
}

// FoundationRoutes returns the complete route set currently available in the foundation HTTP layer.
func FoundationRoutes(info runtime.VersionInfo, identitySnapshots ...agentidentity.Snapshot) []RouteDefinition {
	routes := append([]RouteDefinition{}, HealthRoutes(info)...)
	routes = append(routes, VersionRoutes(info)...)
	routes = append(routes, ReadyRoutes(info)...)
	if len(identitySnapshots) > 0 {
		routes = append(routes, IdentityRoutes(info, identitySnapshots[0])...)
	}
	return routes
}
