# Frontend Round 24 (i18n, a11y, mobile, E2E)

This document is the index for the Round 24 frontend work. Each
section stands alone; the goal is that a new contributor can find
every decision in one place.

## Internationalization

* Bundle lives at `src/i18n/index.ts`. Adding a third locale is one
  JSON-shaped object keyed by the existing translation keys.
* `I18nProvider` at `src/i18n/I18nProvider.tsx` exposes
  `{ t, locale, setLocale, locales }` via `useI18n()`.
* The current locale is persisted in `localStorage["dog.locale"]`
  and detected from `navigator.language` on first visit.
* `<LocalePicker />` is mounted in `TopBar.tsx` and is keyboard
  accessible (`aria-label="Language selector"`).

## Accessibility

* All interactive controls receive a `:focus-visible` outline
  (defined globally in `styles/index.css`).
* `prefers-reduced-motion` is honored — animations and transitions
  collapse to 0.01ms when the user has the OS preference set.
* Page landmarks use semantic `<header>`, `<main>`, `<nav>`,
  `<aside>`, `<button>` elements rather than `<div>` soup.
* Form controls have `aria-label` where the label text is not
  visible.

## Mobile

* Custom media query in `styles/index.css` collapses the sidebar
  into an overlay below 768px, hides dense numerical widgets via
  `.desktop-only`, and stacks the top bar into a wrap layout.
* Touch targets are at minimum 32px thanks to `px-2 py-1` button
  padding; the locale picker uses a native `<select>` which the
  OS renders as a touch-friendly wheel on iOS/Android.

## E2E tests

* `frontend/e2e/smoke.mjs` — ten HTTP-level assertions across
  ingest, query, tenant mint, audit, alerts, probe. Zero deps,
  runs in <5s.
* `frontend/e2e/tenants.mjs` — proves a writer for tenant A
  cannot read tenant B's services.
* `frontend/e2e/run.sh` — builds the collector and runs both.

## Coverage

* `npm run test:coverage` — 70%+ floor on the tested core:
  - `hooks/useApiQuery.ts` (100%)
  - `hooks/queries.ts` (85%)
  - `lib/auth.ts` (95%)
  - `lib/fetch.ts` (96%)
  - `components/LoginModal.tsx` (80%)
  - `components/VirtualTable.tsx` (50%)

Pages are excluded: they pull too much UI for the unit-test budget
and are covered by Playwright-style assertions in `e2e/`.
