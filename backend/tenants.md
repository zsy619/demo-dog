# Multi-tenancy (Round 23)

Demo-dog ships tenant isolation across model, store, ingest, and
HTTP layer. A tenant is a logical partition: its logs, metrics,
and traces live in dedicated slices of the hot/cold tier and the
1m/5m materialized views, and API keys minted for one tenant
cannot read data from another.

## Concepts

- **Tenant**: identified by a slug like `acme` or `globex`. The
  registry tracks creation time + display name + active flag.
- **Tenant-bound API key**: created via `POST /api/tenants/<id>/keys`.
  The plaintext is returned exactly once. The key is registered
  in the auth layer with the tenant binding, so every request it
  carries stamps `X-Dog-Tenant: <id>`.
- **Unbound API key**: traditional key with no tenant binding; may
  impersonate any tenant by passing `?tenant=<id>`. Used by
  platform admins.

## Resolution rules

For every request, the handler resolves a tenant in this order:

1. `X-Dog-Tenant` header (stamped by the auth middleware when the
   key is bound) — cannot be overridden.
2. `?tenant=...` query parameter (used by admins to impersonate).

Non-admin keys cannot escape their tenant even by spoofing headers
or query params, because step 1 is enforced server-side.

## Persistence

The store persists across restarts via a write-ahead log:

```
./dog-collector -snapshot /var/lib/dog-collector/snap.bin \
                -wal /var/lib/dog-collector/wal.bin
```

On startup the snapshot restores the in-memory tier, then the WAL
is replayed on top so any in-flight writes since the last snapshot
are recovered. Every 5 minutes (configurable via `-persist-interval`)
the collector snapshots again and rotates the WAL.

WAL format: 8-byte magic + 4-byte version + 4-byte op + 4-byte
length + gob-encoded payload. The format is forward-compatible (unknown
op codes are skipped) and self-repairing (a truncated tail is truncated
on reopen).

## Endpoints

| Method | Path | Role | Purpose |
|---|---|---|---|
| GET    | /api/tenants | admin | List tenants |
| POST   | /api/tenants | admin | Create tenant |
| POST   | /api/tenants/`<id>`/keys | admin | Mint key (returns plaintext once) |

## CLI

Seed at startup:

```bash
./dog-collector -tenants "acme:Acme Corp,globex:Globex" \
                -api-keys "admin:admin:ops,acme_writer:writer::acme"
```

The 4-segment spec `<key>:<role>:<label>:<tenant>` binds a key to
a tenant at registration. An empty label is fine.

## Tests

```bash
cd backend && go test -race -count=1 ./...
```

`./xdata/tenants` covers create / duplicate / mint / lookup.
`./xdata/store` covers WAL round-trip + repair + rotate +
replay-on-restart. `./xnet/api` covers tenant isolation in
filter and admin endpoints.
