// Smoke test: ensure the SDK does not crash on construction.
// Real E2E coverage lives in /frontend/e2e/sdk-node.mjs.
const assert = require("node:assert");
const { Client } = require("./index.js");

const c = new Client({ baseUrl: "http://127.0.0.1:1", apiKey: "test", flushIntervalMs: 0 });
c.log("hello", "INFO", { user_id: "u-42" });
c.counter("metric.count", 1);
c.histogram("metric.duration_ms", 12.3, { region: "us-east-1" });
c.span("trace-1", "span-1", "GET /x", 42, "ok");
assert.strictEqual(c.logs.length, 1);
assert.strictEqual(c.metrics.length, 2);
assert.strictEqual(c.spans.length, 1);
console.log("OK");
