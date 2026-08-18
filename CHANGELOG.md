# Changelog

All notable changes to demo-dog are documented here. The format is loosely
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the
project adheres to [Semantic Versioning](https://semver.org/) once it leaves
demo state.

## [Unreleased] — Round 21 (frontend hardening)

### Added

- Frontend: login modal (`components/LoginModal.tsx`) with API-key entry
  and localStorage persistence (`lib/auth.ts`, `lib/fetch.ts`).
- Frontend: WebSocket auth via `?api_key=` query string
  (`lib/ws.ts`) since browsers cannot set arbitrary headers on `WebSocket`.
- Frontend: TanStack Query (`@tanstack/react-query`) layer with
  `AbortController`-aware fetch wrapper. Centralised query keys in
  `hooks/queries.ts`; existing `useEffect` polling in TopBar refactored
  onto the new layer.
- Frontend: route-level `React.lazy` + `Suspense` for all 11 pages so a
  user who never opens Logs never downloads it.
- Frontend: `VirtualTable` / `VirtualList` via `@tanstack/react-virtual`,
  flat-rendering below 1k rows, virtualised above. Wired into `Logs`.
- Frontend: tenant switcher in `Sidebar`. Edits `auth.tenantId` and
  triggers a refetch of services via `useServices(tenant)`.
- Frontend: Vitest + React Testing Library scaffolding
  (`vitest.config.ts`, `src/test-setup.ts`, `npm test`).
- Frontend: ESLint 9 flat config + Prettier 3 (`.prettierrc.json`,
  `eslint.config.js`, `npm run lint` / `npm run format`).
- Repo: `LICENSE` (MIT) at root.
- Repo: this `CHANGELOG.md`.

### Changed

- `frontend/src/lib/api.ts` rewired through `apiFetch` so every call
  automatically attaches the API key and tenant id; legacy `api` alias
  preserved for callers that have not migrated.
- `frontend/src/App.tsx` is now the lazy-loaded route host; pages live
  in their own chunks.
- `frontend/vite.config.ts` adds `manualChunks` (react + query) and
  reports per-chunk size to keep the entry under 100 KiB gz.

### Verified

- `npm run typecheck` — green
- `npm run build` — green; entry chunk **12.82 KiB gz**, total first
  paint ≈ **70 KiB gz**
- `npm test` — 20 / 20 green (auth, fetch, LoginModal, VirtualTable)
- `npm run lint` — 0 errors, 19 warnings (all pre-existing dead vars)

## [0.1.0] — Round 20 (backend hardening)

### Added

- Backend: API-key auth (collector middleware + `WithAuthToken` /
  `WithAPIKey` SDK options).
- Backend: TLS, CORS whitelist, WS origin check, per-IP rate limit,
  `MaxBytesReader` 4 MiB.
- Backend: TenantID threading from SDK to store; `?tenant=` query
  parameter; `X-Tenant-Id` header.
- Backend: gob-encoded snapshot persistence (`-snapshot` flag, atomic
  save on graceful shutdown + fatal listen error).
- Backend: `/metrics` exposes pool counters and Go runtime gauges.
- Backend: OTel histogram buckets transparent end-to-end
  (`WithHistogramBuckets` SDK option, `/api/histogram/otel` endpoint).
- Docs: `ROADMAP_30D.md` 30-day execution plan; README
  "Production readiness" section.

## [0.0.1] — Initial demo

Initial end-to-end demo: in-memory Doris sim, simplified OTel envelope,
react + tailwind frontend, 17 SDK examples.
