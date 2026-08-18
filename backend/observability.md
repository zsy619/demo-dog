# Self-observability (Round 22.5)

Four additions turn the collector from a black box into a
self-monitoring system:

## 1. Per-handler latency histogram

Every request records its wall-clock duration in
dog_request_duration_seconds, labelled by HTTP method and a
cardinality-controlled route template. The histogram is exposed
through /metrics alongside the existing counters.

Cardinality control: route templates collapse noisy path segments
into {name} / {id} so a single metric series survives thousands
of distinct services.

## 2. pprof (token-gated)

dog-collector -addr :18080 -pprof-token secret123
curl "http://localhost:18080/debug/pprof/?token=secret123"
curl "http://localhost:18080/debug/pprof/heap?token=secret123"

Without the token (or with a wrong one) every pprof path returns
403, so a leaked URL is not enough to drive heap dumps.

## 3. Blackbox probe (/api/probe)

Returns 200 OK with engine stats (logs / metrics / spans accepted,
queries served, uptime). Public endpoint, no auth required, so K8s
readinessProbe can hit it without rotating keys.

## 4. Self-tracing

dog-collector -self-trace

When enabled, every request produces an OTLP span (POSTed back to
the local /api/ingest/otlp). The collector graphs its own latency
without an external SDK in the same process. Disabled by default
because it adds one POST per request.

## Layering (outer -> inner)

withCORS -> audit -> rateLimit -> selfTrace -> per-handler latency
  -> [pprof + auth.Middleware] -> applyRoleGates -> mux

pprof mounts OUTSIDE auth so it never has to deal with API keys.
The audit middleware records writes only (recordReads=false).

## Tests

cd backend && go test -race -count=1 ./...

Covers the histogram series / vector, route templating, and the
existing audit middleware.
