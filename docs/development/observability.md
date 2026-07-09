# Observability Foundation

Version: v0.1.37

EPIC 11 introduces local observability contracts for the Auren Transfer Agent.

## Capabilities

The foundation exposes seven observability capabilities:

- Prometheus text exposition;
- Grafana dashboard JSON export;
- local tracing recorder;
- local audit recorder;
- local alert evaluator;
- local dashboard payload;
- centralized log sink foundation.

These components are local, bounded and offline-compilable. They do not start external exporters, tracing backends, alert notifiers, Grafana services or log shipping loops.

## Routes

When routes are mounted by bootstrap, the observability endpoints are:

```text
GET /metrics
GET /api/v1/observability
GET /api/v1/observability/grafana/dashboard
GET /api/v1/observability/traces
POST /api/v1/observability/traces
GET /api/v1/observability/audit
POST /api/v1/observability/audit
GET /api/v1/observability/alerts
GET /api/v1/observability/logs
POST /api/v1/observability/logs
```

The Prometheus path uses `metrics.path`, which defaults to `/metrics`.

## Security

Authentication follows the same foundation API-key wrapper used by communication routes. If `security.api_key_required=true`, observability routes require the configured key in `security.token_header`.

## Design rule

Observability records local technical state only. It must not own Media Hub workflow decisions, billing decisions, customer policy or provider-specific business logic.
