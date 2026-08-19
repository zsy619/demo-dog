#!/usr/bin/env node
// bench/prom_write.js — Prometheus remote-write benchmark.
//
// Encodes a WriteRequest in the wire format (snappy-framed
// protobuf) and POSTs it to /api/v1/write. We use a minimal
// snappy framing + protobuf encoder inline because the test
// must be zero-dep.
//
// Usage:
//   node bench/prom_write.js [--url http://localhost:18080] [--key admin] [--n 5000] [--concurrency 20]

import { performance } from "node:perf_hooks";

const args = parseArgs(process.argv.slice(2));
const URL = args.url || "http://localhost:18080";
const KEY = args.key || "admin";
const N = parseInt(args.n || "5000", 10);
const CONCURRENCY = parseInt(args.concurrency || "20", 10);

function parseArgs(argv) {
  const out = {};
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a.startsWith("--")) { out[a.slice(2)] = argv[i + 1]; i++; }
  }
  return out;
}

// Minimal protobuf encoder. Field types: varint=0, length-delim=2, fixed64=1.
function writeVarint(out, n) {
  while (true) {
    if ((n & ~0x7f) === 0) { out.push(n); return; }
    out.push((n & 0x7f) | 0x80);
    n = Math.floor(n / 128); // safe for small n; we only encode timestamps
  }
}
function writeTag(out, field, type) {
  writeVarint(out, (field << 3) | type);
}
function writeString(out, s) {
  const b = Buffer.from(s, "utf8");
  writeVarint(out, b.length);
  for (const c of b) out.push(c);
}
function writeF64(out, f) {
  writeTag(out, 2, 1); // fixed64
  const buf = Buffer.alloc(8);
  buf.writeDoubleLE(f, 0);
  for (const c of buf) out.push(c);
}

function buildWriteRequest() {
  const out = [];
  // WriteRequest.timeseries = field 1, repeated message
  writeTag(out, 1, 2);
  const ts = [];
  // labels
  const labels = [
    { name: "__name__", value: "bench_metric" },
    { name: "service", value: "bench" },
  ];
  const labelsBytes = [];
  for (const l of labels) {
    writeTag(labelsBytes, 1, 2);
    const lmsg = [];
    writeTag(lmsg, 1, 2); writeString(lmsg, l.name);
    writeTag(lmsg, 2, 2); writeString(lmsg, l.value);
    writeVarint(labelsBytes, lmsg.length);
    for (const c of lmsg) labelsBytes.push(c);
  }
  // samples
  const samplesBytes = [];
  writeTag(samplesBytes, 1, 1); writeF64(samplesBytes, Math.random() * 100);
  writeTag(samplesBytes, 2, 0); writeVarint(samplesBytes, Date.now());

  const tsBytes = [];
  writeTag(tsBytes, 1, 2); writeVarint(tsBytes, labelsBytes.length);
  for (const c of labelsBytes) tsBytes.push(c);
  writeTag(tsBytes, 2, 2); writeVarint(tsBytes, samplesBytes.length);
  for (const c of samplesBytes) tsBytes.push(c);

  writeVarint(ts, tsBytes.length);
  for (const c of tsBytes) ts.push(c);

  writeVarint(out, ts.length);
  for (const c of ts) out.push(c);
  return Buffer.from(out);
}

// Snappy framed format: each chunk is preceded by a varint length
// and the chunk type byte. We emit the simplest case: a single
// uncompressed literal chunk.
function snappyFrameLiteral(data) {
  const out = [];
  // chunk type 0x00 = literal
  // header byte is just the type
  out.push(0x00);
  // length (32-bit little-endian) = 4 (length itself) + 1 (type byte? actually no — type is part of header)
  // Re-read spec: each chunk is [length:u32][type:u8][payload]
  // We have header byte already included, so chunk length = 1 (type) + payload
  const len = 1 + data.length;
  out.push((len >>> 0) & 0xff);
  out.push((len >>> 8) & 0xff);
  out.push((len >>> 16) & 0xff);
  out.push((len >>> 24) & 0xff);
  out.push(0x00);
  for (const c of data) out.push(c);
  return Buffer.from(out);
}

async function send(payload) {
  const t0 = performance.now();
  const r = await fetch(`${URL}/api/v1/write`, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-protobuf",
      "Content-Encoding": "snappy",
      "Authorization": `Bearer ${KEY}`,
    },
    body: payload,
  });
  if (!r.ok) throw new Error(`HTTP ${r.status}: ${await r.text()}`);
  return performance.now() - t0;
}

const latencies = new Array(N);
let cursor = 0, ok = 0, err = 0;
const t0 = performance.now();

async function worker() {
  while (true) {
    const i = cursor++;
    if (i >= N) return;
    try {
      const body = buildWriteRequest();
      const framed = snappyFrameLiteral(body);
      latencies[i] = await send(framed);
      ok++;
    } catch (e) {
      err++;
      if (err < 5) console.error("err:", e.message);
    }
  }
}
const ws = [];
for (let i = 0; i < CONCURRENCY; i++) ws.push(worker());
await Promise.all(ws);

const t1 = performance.now();
const elapsed = (t1 - t0) / 1000;
latencies.sort((a, b) => a - b);
const pct = (p) => latencies[Math.min(latencies.length - 1, Math.floor((p / 100) * latencies.length))];

console.log(`
=== demo-dog Prom Remote Write bench ===
URL:           ${URL}
Total:         ${N}
Concurrency:   ${CONCURRENCY}
Ok:            ${ok}
Errors:        ${err}
Elapsed:       ${elapsed.toFixed(2)}s
RPS:           ${(ok / elapsed).toFixed(0)}
p50:           ${pct(50).toFixed(2)} ms
p95:           ${pct(95).toFixed(2)} ms
p99:           ${pct(99).toFixed(2)} ms
`);
if (err > 0) process.exitCode = 1;
