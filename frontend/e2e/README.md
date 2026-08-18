# End-to-End Smoke Tests (Round 24)

Round 24 ships two end-to-end smoke tests that exercise the running
stack top-to-bottom without a heavyweight framework:

* `smoke.mjs` — boots `dog-collector` on :18081, runs ~20 real
  HTTP requests against it (ingest, query, admin endpoints,
  audit, alerts, tenants), and exits 0 if every assertion holds.
* `tenants.mjs` — exercises the tenant isolation contract:
  two tenants, two writers, proves a writer for tenant A cannot
  see tenant B's services.

Run them with:

```bash
cd frontend/e2e
./smoke.mjs       # against an already-running collector on :18080
./smoke.mjs 18081 # against :18081
./tenants.mjs
```

Both scripts auto-spawn a collector if the port is free and kill
it on exit. The CI pipeline runs `./smoke.mjs` first and
`./tenants.mjs` second.
