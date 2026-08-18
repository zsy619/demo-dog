# Alerting (Round 22.7)

SLO burn-rate rules with multi-window evaluation, webhook delivery,
and a frontend page that shows active rules + recent fires.

## Concepts

A rule pins a service to an SLO target (e.g. 99% success over 30m).
The engine evaluates two windows on every tick:

- fast window (e.g. 5m) — short-term surge detection. Threshold
  fast_burn defaults to Google SRE 14.4x.
- slow window (e.g. 30m) — long-term drift detection. Threshold
  slow_burn defaults to 1x.

Burn rate is (1 - success_ratio) / (1 - target). A burn rate of
14.4x over 5 minutes means the error budget for the slow window
will be fully consumed in ~5 minutes if the current rate holds.

## Format

YAML:

rules:
  - name: checkout-availability
    service: checkout
    target: 0.99
    window: 30m
    fast_window: 5m
    fast_burn: 14.4
    slow_burn: 1
    severity: critical
    channels:
      - https://hooks.example/incidents

JSON (same shape):

{"rules": [{"name": "x", "target": 0.99, "window": "30m", ...}]}

## CLI

dog-collector -alerts-rules alerts.yaml

Evaluation runs every 30s on a background ticker. Each fire POSTs
the JSON envelope below to every channel URL. Fires dedupe per
(rule, window) for 5 minutes so a single bad minute does not flood
webhooks.

## HTTP API

- GET /api/alerts/rules — active rules (sorted by name)
- GET /api/alerts/fires?n=50 — last N fires from the ring buffer
  (cap 256)

## Frontend

The Alerts page (sidebar: Admin > Alerts) lists rules and the last
50 fires, refreshing every 15 seconds. Read-only by design: rule
edits go through git.

## Tests

cd backend && go test -race -count=1 ./internal/alerts

Covers fire-on-fast-burn, no-fire-when-budget, empty-window skip,
and per-rule dedupe.
