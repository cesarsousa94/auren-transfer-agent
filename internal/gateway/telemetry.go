package gateway

import (
	"context"
	"net/http"
	"time"

	"github.com/cesarsousa94/auren-transfer-agent/internal/mediahub"
)

func (runtime *Runtime) sendHeartbeat(ctx context.Context, state mediahub.NodeState, resolved mediahub.GatewayResolveResult, record SessionRecord, status int, message string) {
	if runtime == nil || runtime.client == nil {
		return
	}
	_ = runtime.client.SendGatewaySessionHeartbeat(ctx, state, mediahub.GatewaySessionHeartbeatPayload{
		SessionID:        resolved.SessionIDOr(record.SessionID),
		Token:            record.Token,
		Kind:             record.Kind,
		ID:               record.ID,
		Extension:        record.Extension,
		Mode:             resolved.NormalizedMode(),
		Status:           "active",
		HTTPStatus:       status,
		BytesSent:        record.BytesSent,
		CurrentEgressBps: runtime.tracker.CurrentEgressBps(),
		Message:          message,
		GeneratedAt:      runtime.now().UTC(),
	})
}

func (runtime *Runtime) closeSession(ctx context.Context, state mediahub.NodeState, resolved mediahub.GatewayResolveResult, record SessionRecord, status int, reason string) {
	if runtime == nil || runtime.client == nil {
		return
	}
	if status == 0 {
		status = http.StatusOK
	}
	_ = runtime.client.CloseGatewaySession(ctx, state, mediahub.GatewaySessionClosePayload{
		SessionID:       resolved.SessionIDOr(record.SessionID),
		Token:           record.Token,
		Kind:            record.Kind,
		ID:              record.ID,
		Extension:       record.Extension,
		Mode:            resolved.NormalizedMode(),
		Status:          "closed",
		HTTPStatus:      status,
		BytesSent:       record.BytesSent,
		DurationSeconds: time.Since(record.StartedAt).Seconds(),
		Reason:          reason,
		GeneratedAt:     runtime.now().UTC(),
	})
}

func (runtime *Runtime) emitEvent(ctx context.Context, state mediahub.NodeState, eventType string, message string, metadata map[string]string) {
	if runtime == nil || runtime.client == nil {
		return
	}
	_ = runtime.client.SendGatewayEvents(ctx, state, mediahub.GatewayEventsPayload{
		Events:      []mediahub.EventPayload{{Level: "warning", Type: eventType, Message: message, Metadata: metadata, CreatedAt: runtime.now().UTC()}},
		GeneratedAt: runtime.now().UTC(),
		Metadata:    map[string]any{"runtime": RuntimeName},
	})
}
