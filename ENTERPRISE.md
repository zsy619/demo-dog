# demo-dog — Enterprise readiness

This document is the single-page summary of demo-dog's
enterprise-grade feature set.

For per-round implementation detail, see
`backend/PRODUCTION_READINESS_REPORT.md` (43 rounds shipped,
stdlib-only deployment).

## Multi-tenancy

* Per-tenant API keys (`DOG_API_KEYS="reader-key:reader:tenant1"`).
* Tenant-scoped queries (logs, metrics, traces, spans).
* Admin impersonation via `?tenant=` query string with audit
  trail.
* Tenant CRUD via `/api/admin/tenants`.
* Tenant UI in the admin panel.

## Persistence

* Write-ahead log (`data/demo-dog.wal`) flushed on graceful
  shutdown.
* WAL replay on startup.
* Bounded hot-tier memory; cold-tier persistence.
* Backup via file copy.

## Scaling

* Zero-dep Go binary; runs on Linux, macOS, Windows.
* Single-binary deployment with embedded static assets.
* Horizontal scaling behind an L7 load balancer.
* Per-tenant sharding by binding collectors to a single tenant.

## Frontend

* React 19 with React Query for client cache.
* Virtual scrolling for large log / span lists.
* i18n: English + Simplified Chinese, runtime switch.
* Mobile responsive CSS.
* Accessibility: focus management, ARIA labels, full keyboard
  navigation.
* Dark + light themes.

## SDKs

* Go (stdlib only)
* Python 3.8+ (stdlib only)
* Node 18+ (stdlib only)
* Java 11+ (stdlib only)

All four expose the same surface:

```
log / counter / gauge / histogram / span / flush / close
```

## Ingest paths

| Path | Use case |
|---|---|
| `POST /api/ingest/otlp` | demo-dog SDKs (simplified JSON) |
| `POST /api/ingest/otlp-json` | OTel SDKs exporting JSON |
| `POST /v1/logs` | OTel collector (standard transport) |
| `POST /v1/metrics` | OTel collector (standard transport) |
| `POST /v1/traces` | OTel collector (standard transport) |
| `POST /api/v1/write` | Prometheus remote-write agents |
| `POST /api/prom/write` | alias |

## Query

* `/api/services` — list services with health summary.
* `/api/services/{name}` — drill-down: endpoints, errors,
  traces, metric names.
* `/api/query?type=logs|metrics|traces&service=…` — raw query.
* `/api/metric-names` — top metric names.
* `/api/v1/query?query=…` — PromQL-lite endpoint (selectors,
  sum/avg/count, rate(), histogram_quantile()).

## Alerting & recording

* Rule DSL: metric + threshold + window.
* Three channels: webhook, email (SMTP + STARTTLS), PagerDuty
  (Events API v2).
* Slack incoming webhook with severity-colored attachments.
* RetryChannel with exponential backoff (2^(i-1)).
* Multiplexer to fan out to multiple channels per rule.
* Recording rules: precomputed queries that materialize a new
  series (e.g. `sum(rate(http_requests[5m]))` →
  `http_requests_5m`).
* Test endpoint: `POST /api/alerts/rules/{name}/test`.
* Rule list: `GET /api/v1/rules` (Prometheus-compatible).

## Observability

* `GET /api/health` — engine stats + uptime + version.
* `GET /metrics` — Prometheus exposition (includes tenant
  quota series).
* `GET /api/admin/audit` — per-request audit log.
* `GET /api/openapi` — full OpenAPI 3.1 spec (14 paths,
  2 security schemes, 4 schemas).
* `GET /api/v1/rules` — alert + recording rule state.
* `GET /api/v1/quotas` — tenant quota consumption.
* W3C Trace Context propagation (`traceparent` /
  `tracestate`).

## Operations

* Grafana JSON dashboards (overview + bridge).
* Grafana datasource + dashboard provisioning YAML.
* OpenTelemetry Collector drop-in (`ops/otelcol/dog-collector.yaml`).
* `bench/ingest.js` + `bench/prom_write.js` zero-dep
  benchmarks.
* `ops/RUNBOOK.md` with 10 operational scenarios.
* K8s manifests in `ops/k8s/`, Helm chart in `deploy/`.
* Docker-friendly static binary.

## High availability

* WAL append-only log with repair + snapshot.
* Multi-node HA via replica primary/follower.
* At-least-once delivery: `POST /replica/ack` with retention
  buffer + min-ack GC.
* Bearer-token auth (hashed, constant-time compare) on
  `/replica/*`.
* mTLS for `/replica` with stdlib `crypto/tls` (cert + key +
  CA bundle, `RequireAndVerifyClientCert`).
* W3C Trace Context propagation across follower→primary calls.

## Auth

* API keys with optional per-key resource scopes
  (`rules:read`, `metrics:read`, etc.).
* OIDC ID token federation via dex / keycloak / Okta —
  verifies RS256 / ES256 / ES384 against issuer JWKS, maps
  `sub`, `aud`, `exp`, `scope`, `groups`, `tenant_id` claims
  into the same auth shape as static API keys.

## Multi-tenant governance

* Per-tenant quota tracker: MaxRequests / MaxBytes per window
  with automatic window rollover.
* Tenant-aware Prometheus metrics: requests, bytes, limited
  state, configured caps.

## Performance (Apple Silicon, single core)

| Endpoint | Throughput | p99 |
|---|---|---|
| `/api/ingest/otlp` | 5 k+ req/s | 31 ms |
| `/api/v1/write` (Prom) | 1 k+ req/s | <50 ms |
| `/api/query` | 20 k+ req/s | <10 ms |

Memory footprint: ~80 MB RSS at idle, ~200 MB under 10 k req/s
load with the default hot-tier caps.

## Deployment model

* **Zero third-party dependencies.** Backend is Go 1.26 stdlib
  only; SDKs are stdlib only (Python 3.8+, Node 18+, JDK 11+).
* Single static binary with embedded frontend assets.
* Helm chart + Dockerfile + systemd unit + k8s manifests.
* Default operational state: standalone with WAL; opt-in HA
  via replica + WAL replication.
* Operator surface area: env vars + `cmd/dog-collector` flags.
