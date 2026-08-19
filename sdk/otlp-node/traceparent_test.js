// W3C traceparent injection: when setCurrent is called, the next flush
// should attach a traceparent header to the ingest request.
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
  const c = new Client({ baseUrl: "http://127.0.0.1:" + port, apiKey: "x", flushIntervalMs: 0 });
  c.setCurrent("4bf92f3577b34da6a3ce929d0e0e4736", "00f067aa0ba902b7");
  c.counter("m", 1);
  await c.flush();
  srv.close();
  const tp = srv._gotHeaders && srv._gotHeaders["traceparent"];
  assert.ok(tp, "traceparent header missing");
  assert.ok(tp.startsWith("00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-"), "unexpected: " + tp);
  console.log("OK traceparent");
});
