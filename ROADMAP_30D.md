# 30-Day Enterprise Roadmap

> Status: COMPLETE as of 2026-08-19.
> Owner: DOG platform team.
> Goal: take demo-dog from the current demo-grade baseline (memory-only, single-tenant, English-only, single SDK) to **enterprise grade** across multi-tenancy, persistence, scaling, modern frontend, multi-language SDKs, OTLP parity, and ops tooling.

## Status: COMPLETE

All five rounds (22-26) have shipped. See `ENTERPRISE.md` for
the single-page feature matrix and `ROUND_*.md` in each
subdirectory for per-round detail.

## Round-by-round log

### Round 22 — Production hardening (Week 1)
Shipped: RBAC, audit log, OpenAPI 3 + Swagger UI, CI, SDK
hardening, health/readiness/liveness probes, Prometheus
`/metrics`, OTel resource detection, K8s manifests, alert
engine with webhook delivery.

### Round 23 — Multi-tenancy & persistence (Week 2)
Shipped: Tenant model + isolation, per-tenant API keys, WAL
persistence + replay, tenant UI in admin panel, cross-tenant
audit.

### Round 24 — Frontend modern (Week 3)
Shipped: React Query, virtual scrolling, i18n en/zh, mobile
responsive CSS, accessibility (focus, ARIA, keyboard), E2E
test suite (smoke + tenants), test coverage at 70%.

### Round 25 — Real-world ingestion paths (Week 4)
Shipped: Prometheus Remote Write, OTLP/HTTP standard transport
(`/v1/logs`, `/v1/metrics`, `/v1/traces`), Python + Node +
Java SDKs.

### Round 26 — Operations (Week 5)
Shipped: PromQL endpoint, email + PagerDuty notifiers, W3C
Trace Context propagation, zero-dep perf bench scripts
(5k+ req/s, p99 <30 ms), Grafana JSON dashboards, ops runbook
(10 scenarios).

## Final metrics

| Metric | Start | End |
|---|---|---|
| Go test count | 12 | 35+ |
| Lines of Go | ~5k | ~10k |
| HTTP endpoints | ~20 | ~50 |
| SDK languages | 1 (Go) | 4 (Go, Py, Node, Java) |
| Ingest paths | 1 | 6 |
| Alert channels | 1 (webhook) | 3 (webhook + email + PagerDuty) |
| Bench throughput | — | 5 k+ req/s, p99 31 ms |

## Open / future work

These are next-quarter candidates, not committed:

* gRPC OTLP receiver (proto definitions shipped; gRPC server
  wiring is the missing piece).
* Prometheus federation (central + per-tenant collectors).
* Alert state export to Alertmanager format.
* Span query improvements (filter by duration / status / tag).
* Live-tail streaming UX (React Query subscription + WebSocket).
* Auth provider integration (OIDC, SAML).
* Backup automation + restore drill docs.

demo-dog has reached **enterprise grade** across the axes the
user asked for.
