#!/usr/bin/env node
// Round 24 smoke: boot the collector, exercise the full HTTP surface.
import { startCollector, stopCollector, req, assert } from "./lib.mjs";

const PORT = Number(process.argv[2] || 18081);
console.log(`>>> smoke test against :${PORT}`);

const child = await startCollector(PORT, ["-tenants", "acme:Acme"]);
try {
  // 1. health
  let r = await req(PORT, "/api/health");
  assert(r.status === 200, `health ${r.status}`);
  assert(r.body?.status === "ok", "status ok");

  // 2. seed → services non-empty
  r = await req(PORT, "/api/seed?service=checkout&n=20", { token: "admin" });
  assert(r.status === 200, `seed ${r.status}`);

  r = await req(PORT, "/api/services", { token: "admin" });
  assert(r.body.services.length > 0, "services non-empty");

  // 3. ingest OTLP path
  r = await req(PORT, "/api/ingest/otlp", {
    method: "POST",
    token: "admin",
    body: {
      resource_attrs: { "service.name": "smoke" },
      logs: [{ timestamp_ns: Date.now() * 1_000_000, severity_text: "INFO", body: "hello" }],
    },
  });
  assert(r.status === 200 || r.status === 202, `ingest ${r.status}`);
  assert(r.body?.accepted_logs === 1, "ingested 1 log");

  // 4. query logs
  r = await req(PORT, "/api/query?type=logs&service=smoke&limit=5", { token: "admin" });
  assert(r.status === 200, `query ${r.status}`);
  assert(r.body?.type === "logs", "query type logs");

  // 5. tenant list
  r = await req(PORT, "/api/tenants", { token: "admin" });
  assert(r.status === 200, `tenants ${r.status}`);
  assert(r.body.tenants.length >= 1, "acme tenant listed");

  // 6. mint key
  r = await req(PORT, "/api/tenants/acme/keys", {
    method: "POST",
    token: "admin",
    body: { label: "smoke", role: "writer" },
  });
  assert(r.status === 200 || r.status === 201, `mint ${r.status}`);
  assert(r.body.plaintext?.startsWith("dog_"), "plaintext dog_ prefix");

  // 7. alerts engine
  r = await req(PORT, "/api/alerts/rules", { token: "admin" });
  assert(r.status === 200, `alerts ${r.status}`);

  // 8. audit log
  r = await req(PORT, "/api/audit?n=20", { token: "admin" });
  assert(r.status === 200, `audit ${r.status}`);
  assert(r.body?.events?.length > 0, "audit has events");

  // 9. probe
  r = await req(PORT, "/api/probe");
  assert(r.status === 200, `probe ${r.status}`);

  // 10. datasources endpoint
  r = await req(PORT, "/api/datasources", { token: "admin" });
  assert(r.status === 200, `datasources ${r.status}`);
  assert(Array.isArray(r.body?.datasources), "datasources list");

  console.log(">>> smoke OK (10/10)");
} catch (e) {
  console.error("FAIL", e?.message || e);
  process.exitCode = 1;
} finally {
  await stopCollector(child);
}
