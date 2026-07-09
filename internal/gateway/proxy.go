package gateway

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cesarsousa94/auren-transfer-agent/internal/mediahub"
)

func (runtime *Runtime) proxy(writer http.ResponseWriter, request *http.Request, state mediahub.NodeState, resolved mediahub.GatewayResolveResult, record SessionRecord) {
	upstreamMethod := request.Method
	if upstreamMethod == "" {
		upstreamMethod = http.MethodGet
	}
	upstreamRequest, err := http.NewRequestWithContext(request.Context(), upstreamMethod, resolved.UpstreamURL, nil)
	if err != nil {
		http.Error(writer, "create upstream request failed", http.StatusBadGateway)
		runtime.closeSession(request.Context(), state, resolved, record, http.StatusBadGateway, "create_request_failed")
		return
	}
	for key, value := range resolved.Headers {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			upstreamRequest.Header.Set(key, value)
		}
	}
	if request.Header.Get("Range") != "" && upstreamRequest.Header.Get("Range") == "" {
		upstreamRequest.Header.Set("Range", request.Header.Get("Range"))
	}
	if request.UserAgent() != "" && upstreamRequest.Header.Get("User-Agent") == "" {
		upstreamRequest.Header.Set("User-Agent", request.UserAgent())
	}

	response, err := runtime.http.Do(upstreamRequest)
	if err != nil {
		http.Error(writer, "upstream request failed", http.StatusBadGateway)
		runtime.emitEvent(request.Context(), state, "gateway.proxy_failed", "Gateway upstream request failed", map[string]string{"session_id": record.SessionID, "error": err.Error()})
		runtime.closeSession(request.Context(), state, resolved, record, http.StatusBadGateway, "upstream_request_failed")
		return
	}
	defer response.Body.Close()

	copyResponseHeaders(writer.Header(), response.Header)
	if response.ContentLength >= 0 {
		writer.Header().Set("Content-Length", response.Header.Get("Content-Length"))
	}
	status := response.StatusCode
	if request.Method == http.MethodHead {
		writer.WriteHeader(status)
		runtime.sendHeartbeat(request.Context(), state, resolved, runtime.tracker.Snapshot(record.SessionID), status, "head")
		runtime.closeSession(request.Context(), state, resolved, runtime.tracker.Snapshot(record.SessionID), status, "head")
		return
	}

	heartbeatDone := make(chan struct{})
	go runtime.heartbeatLoop(request.Context(), state, resolved, record.SessionID, status, heartbeatDone)
	writer.WriteHeader(status)
	countingWriter := &countingResponseWriter{writer: writer, tracker: runtime.tracker, sessionID: record.SessionID}
	_, copyErr := io.Copy(countingWriter, response.Body)
	close(heartbeatDone)

	final := runtime.tracker.Snapshot(record.SessionID)
	if copyErr != nil {
		runtime.emitEvent(context.Background(), state, "gateway.copy_failed", "Gateway proxy body copy failed", map[string]string{"session_id": record.SessionID, "error": copyErr.Error()})
		runtime.closeSession(context.Background(), state, resolved, final, status, "copy_failed")
		return
	}
	runtime.closeSession(context.Background(), state, resolved, final, status, "completed")
}

func copyResponseHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		canonical := http.CanonicalHeaderKey(key)
		switch canonical {
		case "Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade":
			continue
		}
		for _, value := range values {
			dst.Add(canonical, value)
		}
	}
}

type countingResponseWriter struct {
	writer    http.ResponseWriter
	tracker   *Tracker
	sessionID string
}

func (writer *countingResponseWriter) Write(payload []byte) (int, error) {
	n, err := writer.writer.Write(payload)
	if n > 0 && writer.tracker != nil {
		writer.tracker.AddBytes(writer.sessionID, int64(n))
	}
	if flusher, ok := writer.writer.(http.Flusher); ok {
		flusher.Flush()
	}
	return n, err
}

func (runtime *Runtime) heartbeatLoop(ctx context.Context, state mediahub.NodeState, resolved mediahub.GatewayResolveResult, sessionID string, status int, done <-chan struct{}) {
	interval := resolved.HeartbeatInterval()
	if interval <= 0 {
		interval = parseDurationOr(runtime.cfg.GatewayHeartbeatInterval, 10*time.Second)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	runtime.sendHeartbeat(context.Background(), state, resolved, runtime.tracker.Snapshot(sessionID), status, "started")
	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			runtime.sendHeartbeat(context.Background(), state, resolved, runtime.tracker.Snapshot(sessionID), status, "streaming")
		}
	}
}

func parseDurationOr(value string, fallback time.Duration) time.Duration {
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || duration <= 0 {
		return fallback
	}
	return duration
}
