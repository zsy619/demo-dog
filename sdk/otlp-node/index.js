// @demo-dog/sdk — zero-dep OTLP-style client for the demo-dog collector.
//
// Usage:
//
//   const { Client } = require("@demo-dog/sdk");
//   const c = new Client({ baseUrl: "http://localhost:18080", apiKey: "admin", service: "checkout" });
//   c.log("hello", "INFO");
//   c.counter("orders.placed", 1);
//   c.histogram("checkout.duration_ms", 78.5);
//   c.span("trace-1", "span-1", "GET /checkout", 120, "ok");
//   await c.flush();

const http = require("node:http");
const { URL } = require("node:url");

class Client {
  constructor({
    baseUrl = process.env.DOG_BASE_URL || "http://localhost:18080",
    apiKey = process.env.DOG_API_KEY || "",
    service = "node",
    version = "",
    tenant = process.env.DOG_TENANT || null,
    flushIntervalMs = 1000,
    maxBuffer = 10000,
  } = {}) {
    this.baseUrl = baseUrl.replace(/\/$/, "");
    this.apiKey = apiKey;
    this.service = service;
    this.version = version;
    this.tenant = tenant;
    this.flushIntervalMs = flushIntervalMs;
    this.maxBuffer = maxBuffer;
    this.logs = [];
    this.metrics = [];
    this.spans = [];
    this._timer = null;
    this._closed = false;
    if (flushIntervalMs > 0) {
      this._timer = setInterval(() => this.flush().catch(() => {}), flushIntervalMs);
      this._timer.unref();
    }
  }

  log(body, severity = "INFO", attributes) {
    this.logs.push({
      timestamp_ns: Date.now() * 1_000_000,
      severity_text: severity,
      body,
      attributes: attributes || {},
    });
    this._maybeDrop();
  }

  counter(name, value, attributes) {
    this.metrics.push({
      timestamp: Date.now(),
      name,
      value: Number(value),
      attributes: attributes || {},
    });
    this._maybeDrop();
  }

  histogram(name, value, attributes) { this.counter(name, value, attributes); }

  span(traceId, spanId, name, durationMs, status = "ok", service, parentSpanId) {
    const now = Date.now();
    this.spans.push({
      trace_id: traceId,
      span_id: spanId,
      parent_span_id: parentSpanId || "",
      service: service || this.service,
      name,
      start_unix_nano: now * 1_000_000,
      duration_ns: Math.round(durationMs * 1_000_000),
      status,
    });
    // Cache the most recent trace + span so the next flush can emit a
    // W3C traceparent header. Applications that have their own
    // tracing context should call .setCurrent(traceId, spanId) instead.
    this._currentTraceId = traceId;
    this._currentSpanId = parentSpanId || spanId;
    this._maybeDrop();
  }

  setCurrent(traceId, spanId) {
    this._currentTraceId = traceId;
    this._currentSpanId = spanId;
  }

  async flush() {
    if (this._closed) return;
    const logs = this.logs; this.logs = [];
    const metrics = this.metrics; this.metrics = [];
    const spans = this.spans; this.spans = [];
    if (!logs.length && !metrics.length && !spans.length) return;
    const body = {
      resource_attrs: {
        "service.name": this.service,
        "service.version": this.version,
      },
      logs,
      metrics,
      spans,
    };
    if (this.tenant) body.tenant_id = this.tenant;
    await this._post("/api/ingest/otlp", body);
  }

  close() {
    this._closed = true;
    if (this._timer) clearInterval(this._timer);
    return this.flush();
  }

  _maybeDrop() {
    const total = this.logs.length + this.metrics.length + this.spans.length;
    if (total > this.maxBuffer) {
      while (this.logs.length + this.metrics.length + this.spans.length > this.maxBuffer) {
        if (this.logs.length) this.logs.shift();
        else if (this.metrics.length) this.metrics.shift();
        else if (this.spans.length) this.spans.shift();
        else break;
      }
    }
  }

  _post(path, body) {
    const u = new URL(this.baseUrl + path);
    const data = Buffer.from(JSON.stringify(body), "utf-8");
    const headers = {
      "Content-Type": "application/json",
      "Content-Length": data.length,
      "Authorization": `Bearer ${this.apiKey}`,
    };
    // Inject a W3C traceparent when the client was started with one.
    // The span API records both trace and parent span ids; we surface
    // the most recently recorded pair as the active trace context.
    if (this._currentTraceId && this._currentSpanId) {
      headers["traceparent"] = `00-${this._currentTraceId}-${this._currentSpanId}-01`;
    }
    return new Promise((resolve, reject) => {
      const req = http.request({
        method: "POST",
        hostname: u.hostname,
        port: u.port || 80,
        path: u.pathname,
        headers,
        timeout: 5000,
      }, (res) => {
        res.on("data", () => {});
        res.on("end", () => resolve(res.statusCode));
      });
      req.on("error", reject);
      req.on("timeout", () => { req.destroy(new Error("timeout")); });
      req.write(data);
      req.end();
    });
  }
}

module.exports = { Client };
