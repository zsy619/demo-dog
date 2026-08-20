# Frontend / SDK vs Backend Audit

After Rounds 1-52. Compares frontend, SDKs, and the API surface against the backend Go module.

Legend: matches, partial, missing.

## 1. API endpoint coverage

Backend mounts 37 main routes + 11 pprof + 3 debug + replica sub-routes.

| Backend route | Frontend | Node | Python | Go | Status |
|---|---|---|---|---|---|
| GET /api/health | yes | n/a | n/a | n/a | ok |
| GET /api/services | yes | n/a | n/a | n/a | ok |
| GET /api/services/{name} | yes | n/a | n/a | n/a | ok |
| GET /api/services/{name}/detail | yes | n/a | n/a | n/a | ok |
| GET /api/query | yes | n/a | n/a | n/a | ok |
| GET /api/datasources | yes | n/a | n/a | n/a | ok |
| GET /api/dashboards | yes | n/a | n/a | n/a | ok |
| GET /api/dashboards/{id}/panels | yes | n/a | n/a | n/a | ok |
| POST /api/ingest/otlp | yes | yes | yes | yes | ok |
| POST /api/ingest/otlp-json | exposed but unused | n/a | n/a | yes | partial |
| GET /api/stream | yes (hook) | n/a | n/a | n/a | ok |
| POST /v1/logs | none | none | none | none | gap (canonical OTLP) |
| POST /v1/metrics | none | none | none | none | gap |
| POST /v1/traces | none | none | none | none | gap |
| GET /api/v1/series | none | n/a | n/a | n/a | unused |
| GET /api/v1/metadata | none | n/a | n/a | n/a | unused |
| GET /api/v1/query (PromQL) | none | n/a | n/a | n/a | unused |
| POST /api/v1/write | none | n/a | n/a | n/a | unused |
| POST /api/prom/write | none | n/a | n/a | n/a | unused |
| GET /api/seed | yes | n/a | n/a | n/a | ok |
| GET /api/seed/stream | yes | n/a | n/a | n/a | ok |
| GET /api/ingest/recent | yes | n/a | n/a | n/a | ok |
| GET /api/labels | yes | n/a | n/a | n/a | ok |
| GET /api/service-map | yes | n/a | n/a | n/a | ok |
| GET /api/traces/{id} | yes | n/a | n/a | n/a | ok |
| GET /api/qps | yes | n/a | n/a | n/a | ok |
| GET /api/histogram | yes | n/a | n/a | n/a | ok |
| GET /api/histogram/otel | none | n/a | n/a | n/a | unused |
| GET /api/severity | yes | n/a | n/a | n/a | ok |
| GET /api/snapshot | yes | n/a | n/a | n/a | ok |
| GET /api/metric-names | yes | n/a | n/a | n/a | ok |
| GET /api/export | yes (URL helper) | n/a | n/a | n/a | ok |
| GET /api/audit | none | n/a | n/a | n/a | gap (no audit page) |
| GET /api/audit/stats | none | n/a | n/a | n/a | gap |
| GET /api/keys | none | n/a | n/a | n/a | gap |
| GET /api/probe | none | n/a | n/a | n/a | gap |
| GET/POST /api/alerts/rules | read only | n/a | n/a | n/a | partial (no mutations) |
| GET/POST/PUT/DELETE /api/v1/rules | none | n/a | n/a | n/a | gap |
| GET /api/alerts/fires | yes | n/a | n/a | n/a | ok |
| GET/POST /api/tenants | create + list only | n/a | n/a | n/a | partial (no delete/update) |
| POST /api/tenants/{id}/keys | mint only | n/a | n/a | n/a | partial (no list/rotate/revoke) |
| GET /metrics | yes | n/a | n/a | n/a | ok |

## 2. Backend modules with NO frontend surface

| Module | Status |
|---|---|
| xcore/audit | no page |
| xsecure/auth/oidc | LoginModal not bound to OIDC |
| xsecure/auth/middleware | only X-API-Key sent, no bearer |
| xcache/breaker | no surface |
| xcache/ratelimit | no surface |
| xnet/webhook | no surface |
| xdata/retention | no surface |
| xflow/alerts/slo | no surface |
| xdata/replica | no surface |
| xdata/replica/tracing | SDKs do not propagate W3C |
| xdata/store/backup | no surface |
| xdata/store/migrate | no surface |
| xcore/health | admin port not exposed |
| xsecure/rbac | not wired to UI |
| xnet/oauth | no surface |
| xsecure/secretrot | no surface |
| xdata/vault | no surface |
| xdata/feature | no surface |
| xsecure/session | no surface |

## 3. Type drift

backend/xnet/api/otlp_http.go decodes JSON envelope. Field shapes match frontend MetricPoint/LogRecord. ✅

## 4. SDK ingest path consistency

| SDK | Endpoint | Auth |
|---|---|---|
| Go | /api/ingest/otlp | Bearer |
| Node | /api/ingest/otlp | Bearer |
| Python | /api/ingest/otlp | Bearer |
| Java | folder only | n/a |

3 active SDKs agree. Java is stubbed.

## 5. Frontend type duplication

AlertRule and AlertFire are declared 3 times in frontend/src/types/api.ts. Should be once.

## 6. Auth header alignment

Backend accepts Bearer / mTLS / Bearer-OIDC. Frontend sends only X-API-Key. Frontend should also send Bearer.

## 7. Revision plan

P1: close ops-blocking gaps

- frontend: audit/probes/webhooks/retention/replica/circuit/ratelimit pages + lib helpers
- frontend: /api/v1/rules mutations
- frontend: tenant lifecycle (DELETE/PATCH)
- frontend: tenant keys lifecycle (list/rotate/revoke)
- frontend: dedupe AlertRule/AlertFire
- frontend: send Bearer in addition to X-API-Key

P2: SDK parity

- Java SDK: implement or delete
- Go SDK: W3C trace context propagation in Exporter
- Node/Python SDK: W3C traceparent out of box

P3: observability surfaces

- frontend: SLO budgets page
- frontend: retention tier table on Tenants
- frontend: replica state on Overview
- frontend: webhook subscribers + DLQ page
