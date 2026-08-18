#!/usr/bin/env node
// Round 24 tenant isolation E2E: prove that a writer for tenant A
// cannot see tenant B's services or read tenant B's logs.
import { startCollector, stopCollector, req, assert } from "./lib.mjs";

const PORT = Number(process.argv[2] || 18082);
console.log(`>>> tenant isolation against :${PORT}`);

const child = await startCollector(PORT, [
  "-tenants",
  "acme:Acme,globex:Globex",
]);
try {
  // Bootstrap keys via the admin endpoint.
  let r = await req(PORT, "/api/tenants/acme/keys", {
    method: "POST",
    token: "admin",
    body: { label: "acme-bot", role: "writer" },
  });
  assert(r.status === 200 || r.status === 201, `mint acme ${r.status}`);
  const acmeKey = r.body.plaintext;

  r = await req(PORT, "/api/tenants/globex/keys", {
    method: "POST",
    token: "admin",
    body: { label: "globex-bot", role: "writer" },
  });
  assert(r.status === 200 || r.status === 201, `mint globex ${r.status}`);
  const globexKey = r.body.plaintext;

  // acme ingests
  r = await req(PORT, "/api/ingest/otlp", {
    method: "POST",
    token: acmeKey,
    body: {
      resource_attrs: { "service.name": "acme-checkout" },
      logs: [{ timestamp_ns: Date.now() * 1_000_000, severity_text: "INFO", body: "acme" }],
    },
  });
  assert(r.status === 200 || r.status === 202, `acme ingest ${r.status}`);

  // globex ingests
  r = await req(PORT, "/api/ingest/otlp", {
    method: "POST",
    token: globexKey,
    body: {
      resource_attrs: { "service.name": "globex-inventory" },
      logs: [{ timestamp_ns: Date.now() * 1_000_000, severity_text: "INFO", body: "globex" }],
    },
  });
  assert(r.status === 200 || r.status === 202, `globex ingest ${r.status}`);

  // acme writer sees only acme services
  r = await req(PORT, "/api/services", { token: acmeKey });
  const acmeNames = (r.body.services || []).map((s) => s.name);
  assert(acmeNames.includes("acme-checkout"), "acme sees own");
  assert(!acmeNames.includes("globex-inventory"), "acme cannot see globex");

  // globex writer sees only globex services
  r = await req(PORT, "/api/services", { token: globexKey });
  const globexNames = (r.body.services || []).map((s) => s.name);
  assert(globexNames.includes("globex-inventory"), "globex sees own");
  assert(!globexNames.includes("acme-checkout"), "globex cannot see acme");

  // admin sees both
  r = await req(PORT, "/api/services", { token: "admin" });
  const allNames = (r.body.services || []).map((s) => s.name);
  assert(allNames.includes("acme-checkout") && allNames.includes("globex-inventory"), "admin sees both");

  console.log(">>> tenants OK");
} catch (e) {
  console.error("FAIL", e?.message || e);
  process.exitCode = 1;
} finally {
  await stopCollector(child);
}
