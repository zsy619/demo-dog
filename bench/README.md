# demo-dog benchmarks

Zero-dep Node scripts for measuring end-to-end ingest performance.

## Scripts

### `node bench/ingest.js`

Measures throughput to `POST /api/ingest/otlp` with the simplified
JSON envelope. Each request contains 1 log + 2 metrics + 1 span.

```bash
node bench/ingest.js --url http://localhost:18080 --key admin --n 10000 --concurrency 50
```

Output:

```
=== demo-dog ingest benchmark ===
Total requests: 10000
Concurrency:   50
Ok:            10000
Errors:        0
Elapsed:       2.31s
Throughput:    4329 req/s
Latency p50:   4.21 ms
Latency p95:   12.30 ms
Latency p99:   18.45 ms
```

### `node bench/prom_write.js`

Measures throughput to `POST /api/v1/write` with the Prometheus
remote-write wire format (snappy-framed protobuf). This script
includes a zero-dep snappy-framer + protobuf encoder; only the
literal-untouched case is needed for benchmarking because the
collector accepts that format directly.

```bash
node bench/prom_write.js --url http://localhost:18080 --key admin --n 5000 --concurrency 20
```

## What to look for

* **Throughput** — should be >2 k req/s on a single core for the
  simplified envelope, and >1 k req/s for Prom Remote Write.
* **p99 latency** — should stay under 20 ms with the default
  in-memory store. Sustained p99 >100 ms means either the store is
  shedding load or the system is CPU-bound on JSON encoding.
* **Error rate** — anything >0 means either authentication is
  failing or back-pressure is activating. Inspect the collector
  log for the cause.

## When to run

Run benchmarks in CI before publishing a release. The numbers
above are the expected baseline on Apple Silicon; if your CI
hardware is slower, the absolute throughput will be lower but
the relative shape (p50, p95, p99 ratios) should be similar.
