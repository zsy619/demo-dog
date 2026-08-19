#!/usr/bin/env node
// bench/ingest.js — performance smoke for the demo-dog ingest endpoint.
//
// Usage:
//   node bench/ingest.js [--url http://localhost:18080] [--key admin] [--n 10000] [--concurrency 50]
//
// What it measures:
//   * End-to-end POST latency to /api/ingest/otlp
//   * Throughput at the chosen concurrency level
//   * p50 / p95 / p99 percentiles
//
// The script is deliberately zero-dep (no k6, no vegeta) so it runs
// everywhere Node runs.

import { performance } from "node:perf_hooks";
import { setTimeout as sleep } from "node:timers/promises";

const args = parseArgs(process.argv.slice(2));
const URL = args.url || "http://localhost:18080";
const KEY = args.key || "admin";
const N = parseInt(args.n || "10000", 10);
const CONCURRENCY = parseInt(args.concurrency || "50", 10);

function parseArgs(argv) {
  const out = {};
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a.startsWith("--")) {
      out[a.slice(2)] = argv[i + 1];
      i++;
    }
  }
  return out;
}

function makeBatch(i) {
  const isoNow = new Date().toISOString();
  return {
    resource_attrs: { "service.name": "bench" },
    logs: [
      { timestamp: isoNow, severity: "INFO", body: `msg-${i}` },
    ],
    metrics: [
      { timestamp: isoNow, name: "bench.counter", value: i },
      { timestamp: isoNow, name: "bench.gauge", value: i / 10 },
    ],
    spans: [
      {
        trace_id: traceIdHex(i),
        span_id: spanIdHex(i),
        service: "bench",
        name: "GET /work",
        start_time: isoNow,
        duration_ms: 1,
        status: "ok",
      },
    ],
  };
}

function traceIdHex(n) {
  const s = n.toString(16).padStart(32, "0");
  return s;
}
function spanIdHex(n) {
  return n.toString(16).padStart(16, "0");
}

async function send(body) {
  const t0 = performance.now();
  const r = await fetch(`${URL}/api/ingest/otlp`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Authorization": `Bearer ${KEY}`,
    },
    body: JSON.stringify(body),
  });
  if (!r.ok) {
    const t = await r.text();
    throw new Error(`HTTP ${r.status}: ${t}`);
  }
  return performance.now() - t0;
}

const latencies = new Array(N);
let cursor = 0;
let okCount = 0;
let errCount = 0;
const t0 = performance.now();

async function worker(id) {
  while (true) {
    const i = cursor++;
    if (i >= N) return;
    try {
      const lat = await send(makeBatch(i));
      latencies[i] = lat;
      okCount++;
    } catch (e) {
      errCount++;
      if (errCount < 5) console.error(`worker ${id} err:`, e.message);
    }
  }
}

const workers = [];
for (let i = 0; i < CONCURRENCY; i++) workers.push(worker(i));
await Promise.all(workers);

const t1 = performance.now();
const elapsed = (t1 - t0) / 1000;
latencies.sort((a, b) => a - b);

function pct(p) {
  const i = Math.min(latencies.length - 1, Math.floor((p / 100) * latencies.length));
  return latencies[i];
}

const rps = okCount / elapsed;
console.log(`
=== demo-dog ingest benchmark ===
URL:           ${URL}
Total requests: ${N}
Concurrency:   ${CONCURRENCY}
Ok:            ${okCount}
Errors:        ${errCount}
Elapsed:       ${elapsed.toFixed(2)}s
Throughput:    ${rps.toFixed(0)} req/s
Latency p50:   ${pct(50).toFixed(2)} ms
Latency p95:   ${pct(95).toFixed(2)} ms
Latency p99:   ${pct(99).toFixed(2)} ms
Latency max:   ${latencies[latencies.length - 1].toFixed(2)} ms
`);

if (errCount > 0) process.exitCode = 1;
