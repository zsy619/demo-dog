# 30-Day Enterprise Roadmap

> Status: drafted 2026-08-18.
> Owner: DOG platform team.
> Goal: take demo-dog from the current demo-grade baseline (memory-only, single
> tenant, no auth) to a production-grade foundation that a small team can run
> for a real internal workload.

## Round 22 — Enterprise hardening (DONE)

| Slice | Task | Status |
|---|---|---|
| 22.1 | RBAC + audit log + multi API key (admin/writer/reader) | done |
| 22.2 | OpenAPI 3.1 spec at `backend/openapi.yaml` (30 paths) | done |
| 22.3 | CI/CD (GH Actions: backend race tests, frontend vitest + typecheck + build, multi-arch docker buildx, SemVer release) | done |
| 22.4 | SDK env config + buffer cap (drop-oldest) + PII redactor + tail sampler | done |
| 22.5 | Self-tracing + pprof (token-gated) + blackbox probe + per-handler latency histogram | done |
| 22.6 | K8s manifests + Helm chart (Deployment, Service, HPA, PDB, PVC, ConfigMap, Ingress, ServiceMonitor) | done |
| 22.7 | Alert rules engine + SLO burn-rate (multi-window) + webhook + frontend Alerts page | done |

This roadmap consumes the audit reports on disk:

- `backend/PRODUCTION_READINESS_REPORT.md` (P0/P1 issues in Go backend)
- `FRONTEND_PRODUCTION_READINESS_REPORT.md` (frontend gaps)
- `SDK_EVALUATION.md` (long-term SDK coverage)
- `_SUMMARY.md` (cross-cutting priorities)

The phases in this document correspond to those in the current implementation
plan (Phase 0/1 done in commit series; Phase 2/3 documented below).

---

## Day 0 (already shipped) — Demo hardening

### Backend (P0 correctness)

- **Phase 0.1**: histograms now use OTel explicit bucket bounds with linear
  interpolation for quantiles (was a running-mean hack); MV rollups sum
  per-minute/per-five-minute correctly (was an averaging hack).
- **Phase 0.2**: OTel envelope parser decodes `explicitBounds` + `bucketCounts`
  on data points and forwards them on the wire instead of dropping them. SDK
  has `WithHistogramBuckets(...)` so calls aggregate into one OTel histogram
  data point per series per flush (was one emit per observation).
- **Phase 0.3**: removed `log.Fatalf` from `main`; SIGINT/SIGTERM now drains
  the ingest pipeline before exit (was losing in-flight batches).
- **Phase 0.4**: `/metrics` now exposes pool counters
  (`dog_ingest_jobs_*`) and Go runtime metrics (`dog_go_*`).
- **Phase 0.5**: `/api/datasources` reads from a thread-safe
  `datasourceRegistry`; `Server.Datasources().Add(...)` lets plugins register
  real backends.
- **Phase 0.6**: `handleIngest` returns 503 + `Retry-After: 1` when the queue
  is full instead of a synchronous fallback that pinned the request goroutine.

### Backend (P1 platform)

- **Phase 1.1**: API-key auth (`-api-keys` flag or `DOG_API_KEYS` env),
  `Authorization: Bearer <key>` middleware, constant-time verify, public paths
  bypass for `/api/health` and `/metrics`. SDK side: `WithAuthToken` /
  `WithAPIKey`.
- **Phase 1.2**: TLS via `-tls-cert`/`-tls-key`; CORS restricted to
  `-cors-origins` allowlist (rejects unknown origins with 403); WS upgrade
  checks `Origin`; per-IP token-bucket rate limiter
  (`-rate-limit`/`-rate-burst`); `http.MaxBytesReader` caps ingest bodies at
  4 MiB.
- **Phase 1.3**: first-class `TenantID` on `OTLPRequest` / `MetricPoint` /
  `LogRecord` / `SpanRecord` / `ServiceSummary`. `?tenant=` query parameter
  filters list/get. `X-Tenant-Id` header as a fallback. SDK has `WithTenant`.
- **Phase 1.4**: gob-encoded snapshot persistence. `-snapshot /path/to/file`
  restores on startup, saves atomically (write-temp + rename) on graceful
  shutdown or fatal listen error.

---

## Day 1-3 — Production readiness hardening

| Task | Owner | Notes |
|---|---|---|
| Snapshot retention + rotation | backend | Old snapshots get archived every N minutes; only the most recent survives a crash. |
| Structured logging (`slog`) | backend | Replace `log.Printf` with `slog` JSON output so it can be ingested by Loki / Splunk. |
| PII scrubbing on logs | backend | Redact `password=...`, `Bearer ...`, `Authorization`, JWT bodies, emails, before they hit hot tier. |
| Health endpoint expansion | backend | `/api/health?verbose=1` returns per-component readiness (store, hub, pool) so K8s can route traffic. |
| Rate limit per API key | backend | Bucket dimension becomes `(api_key, ip)` so noisy keys do not block other tenants. |
| API key rotation | backend | `Dog-server reload -api-keys file.json` hot reload without restart. |

---

## Day 4-7 — Frontend enterprise gap ✅ **Round 21 (2026-08-18)**

The frontend is the weakest surface per the audit report (no auth UI, no RBAC,
no tests, single bundle). Targeted plan:

| Task | Owner | Notes |
|---|---|---|
| Login / token screen | frontend | ✅ `components/LoginModal.tsx` + `lib/auth.ts` + `lib/fetch.ts`. API key in localStorage; Bearer header on every fetch; `?api_key=` on WS handshake. |
| RBAC roles | frontend | ⏳ Deferred. Viewer/editor/admin is a real-auth concern; out of scope for the single-key demo. |
| Virtualized lists | frontend | ✅ `components/VirtualTable.tsx` via `@tanstack/react-virtual`. Wired into `Logs` (>500 rows). |
| Bundle splitting | frontend | ✅ `vite.config.ts` `manualChunks` (react + query). Entry chunk 12.82 KiB gz; total first paint ≈ 70 KiB gz. |
| Vitest + RTL | frontend | ✅ 20 tests passing across `auth`, `fetch`, `LoginModal`, `VirtualTable`. |
| i18n | frontend | ⏳ Deferred. Demo has a single Mandarin-speaking maintainer; copy stays inline. |
| A11y pass | frontend | ⏳ Partial. `aria-label`, `aria-modal`, `role="grid"`, `aria-rowcount` already wired; full keyboard nav deferred. |
| Tenant switcher | frontend | ✅ `Sidebar` `TenantSwitcher` edits `auth.tenantId`; `useServices(tenant)` refetches. |

Round 21 ship: 8 atomic commits, build/typecheck/test/lint all green, no
regressions in backend or SDK. Items marked ⏳ roll into Round 22+.

---

## Day 8-14 — Observability of the observability

A platform that monitors others must monitor itself ruthlessly.

| Task | Owner | Notes |
|---|---|---|
| Self-tracing | backend | OpenTelemetry tracer around `handleIngest`, `QueryLogs`, etc. Export via the same `/api/ingest/otlp` path (loop-back). |
| Self-profiling | backend | `net/http/pprof` under `/debug/pprof`, gated by `-pprof-token`. |
| Snapshot integrity check | backend | On `LoadFromFile`, hash the contents; refuse mismatched version + log a clear error. |
| Backpressure telemetry | backend | Emit a Gauge per pool: `dog_pool_queue_depth`, `dog_pool_in_flight`. Alert when queue > 80 % full. |
| Latency histogram | backend | Per-handler latency captured with `prometheus.HistogramVec` (or our equivalent). |
| Synthetic probes | infra | Blackbox exporter hitting `/api/health`, `/api/services`, `/metrics` every 10 s. |
| Log volume caps | backend | Per-service log volume cap; above the cap, downgrade to WARN+ only. |

---

## Day 15-21 — Multi-language SDK + remote ingest

| Task | Owner | Notes |
|---|---|---|
| Java starter | sdk-java | Drop-in `OtlpDogHttpSender`; reuse the existing wire format; tested against an in-memory stub server. |
| Python starter | sdk-py | Auto-instrumentation via `opentelemetry-instrument` plus a thin exporter pointing at `/api/ingest/otlp`. |
| Node.js starter | sdk-node | `OTLPTraceExporter` adapter; same approach. |
| Rust starter | sdk-rs | `tracing-opentelemetry` exporter pointed at the wire endpoint. |
| gRPC ingest | backend | Optional `:4317` listener for vanilla OTel collectors; reuses the same `ingest.Normalize`. |
| Versioning | sdk-* | SemVer and a single `COMPATIBILITY.md` describing which backend versions each SDK supports. |
| Interop tests | sdk-* | Run the Go SDK + the Java SDK side-by-side against a real collector and assert the histograms match. |

---

## Day 22-25 — Storage diversity

The current `store.Doris` is in-memory. Production needs persistence + scale.

| Task | Owner | Notes |
|---|---|---|
| Pluggable store driver | backend | `Store` interface; default = in-memory Doris sim, prod = ClickHouse / SQLite / Parquet. |
| ClickHouse driver | backend | Use the official `clickhouse-go` driver. Schema: 3 tables + 2 MVs. |
| Snapshot-to-SQLite | backend | Optional path that mirrors the in-memory state into a SQLite file for crash-only deployments. |
| Compaction worker | backend | Background goroutine that promotes cold tier to disk every N minutes. |
| Cardinality caps | backend | Reject metric names + label combinations beyond a configurable budget (default 50k active series). |

---

## Day 26-28 — Alerting + SLO

| Task | Owner | Notes |
|---|---|---|
| Alert rules engine | backend | YAML/JSON rules: `expr`, `for`, `severity`. Stored in `internal/alert/rules.go`. |
| SLO burn rates | backend | Multi-window burn-rate calculator (1h / 6h / 24h / 72h). |
| Webhook / Slack / PagerDuty | backend | Generic HTTP webhook with templated payload. |
| Alerting UI | frontend | Page that lists firing alerts + recent history + silence control. |

---

## Day 29-30 — Release ops

| Task | Owner | Notes |
|---|---|---|
| Helm chart | infra | `demo-dog-collector` chart: Deployment, Service, ConfigMap for `-api-keys`/`-snapshot`/`-cors-origins`, optional Ingress for TLS. |
| OpenAPI spec | backend | Hand-written spec (no codegen deps) at `backend/openapi.yaml`. CI lints it on every PR. |
| CI pipeline | infra | `go test -race ./...`, `go vet ./...`, `npm run typecheck && npm run build`, `npm run test`. |
| Release artifacts | infra | Multi-arch Docker images (`linux/amd64`, `linux/arm64`); GitHub release with changelog auto-generated from commit messages. |
| Runbook | docs | `RUNBOOK.md` covering every documented `-flag`, common errors, and remediation. |

---

## Acceptance gate

Each phase ships only when:

1. `go test -race ./...` is green.
2. The end-to-end smoke (`scripts/smoke.sh`) is green.
3. The README for that phase is updated.
4. The git history is linear (rebased) with conventional-commit messages.

Failure to meet any criterion blocks the next phase.

---

## Out of scope (post 30 days)

- Real Doris cluster backend (the in-memory simulator stays as a unit-test
  fixture and a low-overhead dev mode).
- Distributed ingest via Kafka — Phase 2 of the platform.
- Custom query language (Doris SQL subset) — separate RFC.
- Trace sampling beyond the head-based sampler already in the SDK.
