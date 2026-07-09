package server

import (
	"encoding/json"
	"net/http"

	agentidentity "github.com/cesarsousa94/auren-transfer-agent/internal/identity"
	"github.com/cesarsousa94/auren-transfer-agent/internal/runtime"
)

const (
	// IdentityRouteName is the canonical diagnostic route name for the identity endpoint.
	IdentityRouteName = "identity"

	// IdentityPath is the canonical HTTP path for Agent identity diagnostics.
	IdentityPath = "/identity"

	// IdentityResponseStatus is the stable status value returned by the foundation identity endpoint.
	IdentityResponseStatus = "ok"
)

// IdentityResponse is the stable technical identity payload returned by the foundation identity endpoint.
//
// It exposes only Agent-local identity metadata. Media Hub registration, cluster registry state,
// authentication, queue ownership and business data remain reserved for later roadmap phases.
type IdentityResponse struct {
	Status               string `json:"status"`
	Name                 string `json:"name"`
	Version              string `json:"version"`
	RuntimeStatus        string `json:"runtime_status"`
	Router               string `json:"router"`
	AgentID              string `json:"agent_id"`
	Fingerprint          string `json:"fingerprint"`
	FingerprintAlgorithm string `json:"fingerprint_algorithm"`
	Hostname             string `json:"hostname"`
	HostnameSource       string `json:"hostname_source"`
	Persistence          string `json:"persistence"`
	StoreSource          string `json:"store_source"`
	StorePath            string `json:"store_path"`
}

// NewIdentityResponse creates the canonical foundation identity API payload.
func NewIdentityResponse(info runtime.VersionInfo, snapshot agentidentity.Snapshot) IdentityResponse {
	return IdentityResponse{
		Status:               IdentityResponseStatus,
		Name:                 info.Name,
		Version:              info.Version,
		RuntimeStatus:        info.Status,
		Router:               RouterKindName(),
		AgentID:              snapshot.AgentID,
		Fingerprint:          snapshot.Fingerprint,
		FingerprintAlgorithm: snapshot.FingerprintAlgorithm,
		Hostname:             snapshot.Hostname,
		HostnameSource:       snapshot.HostnameSource,
		Persistence:          snapshot.Persistence,
		StoreSource:          snapshot.StoreSource,
		StorePath:            snapshot.StorePath,
	}
}

// IdentityHandler returns the foundation Agent identity HTTP handler.
func IdentityHandler(info runtime.VersionInfo, snapshot agentidentity.Snapshot) http.HandlerFunc {
	payload := NewIdentityResponse(info, snapshot)
	return func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", HealthContentType)
		writer.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(writer).Encode(payload)
	}
}

// IdentityRoute returns the canonical identity route definition.
func IdentityRoute(info runtime.VersionInfo, snapshot agentidentity.Snapshot) RouteDefinition {
	return RouteDefinition{
		Name:    IdentityRouteName,
		Method:  http.MethodGet,
		Pattern: IdentityPath,
		Handler: IdentityHandler(info, snapshot),
	}
}

// IdentityRoutes returns the foundation route set introduced by EPIC 4.5.
func IdentityRoutes(info runtime.VersionInfo, snapshot agentidentity.Snapshot) []RouteDefinition {
	return []RouteDefinition{IdentityRoute(info, snapshot)}
}
