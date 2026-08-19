# demo-dog Operations Runbook

This runbook covers the most common operational scenarios for
demo-dog. Each section is a self-contained checklist.

## 1. Server won't start

**Symptom:** `dog-collector` exits immediately with a panic or
returns a port-bind error.

**Checklist:**

* Is the binary up to date? `dog-collector -version`.
* Is the listening port free? `lsof -i :18080`.
* Are environment variables set? `env | grep DOG_`.
* Read the panic trace in stderr — usually a missing required
  env (`DOG_API_KEYS`, `DOG_TENANT`, …) or a malformed
  flag.
* Verify the `data/` directory exists if you enabled
  persistence (Round 23).

## 2. API key rejected

**Symptom:** `POST /api/ingest/otlp` returns 401
`missing or invalid API key`.

**Checklist:**

* Did the key include the role suffix?
  `DOG_API_KEYS="reader-key:reader:tenant-name,writer-key:writer:tenant-name,admin-key:admin:ops"`.
* Did the client send `Authorization: Bearer <key>`?
* Is the key registered in the running server? Restart after
  changing `DOG_API_KEYS`.
* For multi-tenant keys, verify the tenant slug matches the
  target tenant exactly.

## 3. Ingest queue is backed up

**Symptom:** `engine.queue_depth` is climbing; latency p99
crosses 200 ms.

**Checklist:**

* Hit `GET /api/health` and inspect `engine.metrics_accepted`
  vs `engine.metrics_dropped`. Drops indicate overload.
* If drops are non-zero: increase `DOG_INGEST_BUFFER` (default
  10000) or scale out the collector behind a load balancer.
* If drops are zero but queue_depth grows: the consumer (write
  path) is the bottleneck — check disk IO and GC pause times.
* Tail `/var/log/demo-dog.log` for `[DOG]` lines that show
  slow handlers.

## 4. Memory pressure

**Symptom:** RSS climbs steadily; container OOMs.

**Checklist:**

* Confirm the persistence flush is enabled (Round 23). Without
  it, hot-tier data only ever lives in memory.
* Verify the cold-tier cap. `GET /api/health` exposes
  `cold_logs` and `cold_metrics`. If these are growing past
  the configured limit, the persistence path is broken.
* Reduce `DOG_HOT_LOG_CAP`, `DOG_HOT_METRIC_CAP`,
  `DOG_HOT_SPAN_CAP` to bound memory under sustained load.
* Consider sampling: only ingest 1 in N traces via a
  collector-side probability filter.

## 5. Prometheus remote-write agent can't connect

**Symptom:** `404` or `415` from `/api/v1/write`.

**Checklist:**

* Confirm the collector is configured to accept remote-write.
  Round 25 adds the endpoint; verify the version.
* Set the agent's `remote_write.url` to
  `http://collector/api/v1/write` and
  `remote_write.headers.Authorization: Bearer <key>`.
* If the agent uses gzip: the collector accepts `gzip`,
  `snappy`, and `identity`. No action required.
* Snappy framed format: the collector decodes both literal
  and copy-with-offset variants. If the agent uses a
  non-standard extension, contact us.

## 6. Tenant data leakage suspected

**Symptom:** A user sees data they should not.

**Checklist:**

* Verify the user's API key is bound to the correct tenant
  (`role:tenant-slug`).
* Confirm the request path is the read API (`/api/query`,
  `/api/services`) and not the admin paths (`/api/admin/*`).
* Tail the audit log: `GET /api/admin/audit?limit=100`. Look
  for the user's recent requests and confirm the `tenant_id`
  field is set correctly on each.
* If leakage persists, rotate the affected API keys.

## 7. Trace context not propagating

**Symptom:** Spans ingested via `traceparent` don't show the
upstream trace id.

**Checklist:**

* Round 26.3 added W3C trace context. Confirm the collector
  is at that version or later.
* Verify the upstream service sends `traceparent` with the
  correct 32-char trace id and 16-char span id.
* All-zero ids are rejected per spec — regenerate.
* Confirm the upstream sends a non-zero flags byte. Bit 0
  must be 1 for sampled traces.

## 8. Alert delivery failing

**Symptom:** Alerts fire in the engine but no notification
arrives.

**Checklist:**

* Webhook: `curl -X POST <url>` from the collector host to
  confirm network reachability. Verify the webhook responds
  with a 2xx within 5 seconds.
* Email: confirm `DOG_SMTP_HOST`, `DOG_SMTP_USER`,
  `DOG_SMTP_PASS`, `DOG_SMTP_FROM` are set and the SMTP
  server accepts connections from the collector host.
* PagerDuty: confirm the `DOG_PAGERDUTY_KEY` integration
  key has not been revoked in the PagerDuty console.
* For all channels: `GET /api/health` and verify the engine
  has not stopped.

## 9. Backup / restore

The persistence path is a write-ahead-log of OTLP envelopes
flushed to `data/demo-dog.wal` (Round 23). To restore:

* Stop the collector.
* Copy `data/demo-dog.wal` to the new host.
* Restart. The replay path will replay the WAL into the
  hot-tier on startup.

For long-term backups, copy the WAL file off-host every
hour.

## 10. Scaling out

For high-volume deployments:

* Run multiple collectors behind an L7 load balancer.
* Each collector is stateless with respect to queries (the
  hot tier is local). Use the WAL persistence path on each
  to capture per-instance data.
* Federate queries via Prometheus federation: point a central
  Prometheus at each collector and aggregate.
* For multi-tenant deployments, isolate tenants on separate
  collectors. Use the `DOG_TENANT` env to bind each collector
  to one tenant.
