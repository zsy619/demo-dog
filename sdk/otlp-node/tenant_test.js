// X-Tenant-Id header: when the client is constructed with tenant,
// every outbound ingest should attach X-Tenant-Id. The backend reads
// this header to scope traffic before decoding the body.
const assert = require("node:assert");
const http = require("node:http");
const { Client } = require("./index.js");

const srv = http.createServer((req, res) => {
  srv._gotHeaders = req.headers;
  res.writeHead(202);
  res.end();
});
srv.listen(0, async () => {
  const port = srv.address().port;
  const c = new Client({
    baseUrl: "http://127.0.0.1:" + port,
    apiKey: "x",
    tenant: "acme",
    flushIntervalMs: 0,
  });
  c.counter("m", 1);
  await c.flush();
  srv.close();
  const th = srv._gotHeaders && srv._gotHeaders["x-tenant-id"];
  assert.strictEqual(th, "acme", "X-Tenant-Id header missing or wrong: " + th);
  console.log("OK tenant");
});
