# @demo-dog/sdk — Node SDK

Zero-dep OTLP-style client. Requires Node 18+ (uses the built-in
`node:http` and `node:url` modules).

```javascript
const { Client } = require("@demo-dog/sdk");

const c = new Client({
  baseUrl: "http://localhost:18080",
  apiKey: process.env.DOG_API_KEY,
  service: "checkout",
  tenant: "acme",           // optional
  flushIntervalMs: 1000,    // batch every 1s
});

c.log("order placed", "INFO", { user_id: "u-42" });
c.counter("orders.placed", 1);
c.histogram("checkout.duration_ms", 78.5);
c.span("trace-1", "span-1", "GET /checkout", 120, "ok");

await c.flush();
```

## Test

```bash
npm test
```

The smoke test verifies buffer accounting and event shape. Real
network coverage lives in the cross-language E2E in `frontend/e2e/`.
