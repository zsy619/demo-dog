#!/usr/bin/env bash
# scripts/smoke.sh
#
# End-to-end smoke test that exercises the full backend surface against a
# running dog-collector (default :18080). Exits non-zero on the first failure.

set -euo pipefail

API="${API:-http://localhost:18080}"

say() { printf "\033[1;36m[smoke]\033[0m %s\n" "$*"; }
fail() { printf "\033[1;31m[FAIL]\033[0m %s\n" "$*"; exit 1; }
pass() { printf "\033[1;32m[ OK ]\033[0m %s\n" "$*"; }

need() { command -v "$1" >/dev/null 2>&1 || fail "missing $1"; }
need curl
need jq

say "GET ${API}/api/health"
H=$(curl -sSf "${API}/api/health")
echo "$H" | jq -e ".status == \"ok\"" >/dev/null || fail "engine not ok"
pass "engine healthy"

say "GET ${API}/api/services"
S=$(curl -sSf "${API}/api/services")
echo "$S" | jq -e ".count >= 0" >/dev/null || fail "services payload invalid"
pass "services endpoint ok"

say "POST ${API}/api/ingest/otlp"
curl -sSf -X POST -H "Content-Type: application/json" -d "$(cat <<EOF
{
  "resource_attrs": {"service.name": "smoke-test"},
  "logs":   [{"timestamp": "2026-08-18T10:00:00Z", "service": "smoke-test", "severity": "INFO",  "body": "hello"}],
  "metrics":[{"timestamp": "2026-08-18T10:00:00Z", "service": "smoke-test", "name": "requests", "value": 1, "type": "counter"}],
  "spans":  [{"trace_id": "deadbeef", "span_id": "cafebabe", "service": "smoke-test", "start_time": "2026-08-18T10:00:00Z", "duration_ms": 7, "status": "ok"}]
}
EOF
)" "${API}/api/ingest/otlp" | jq -e ".accepted_logs >= 1 and .accepted_metrics >= 1 and .accepted_spans >= 1" >/dev/null \
  || fail "ingest rejected"
pass "otlp ingest accepted"

say "GET ${API}/api/query?type=logs&service=smoke-test"
curl -sSf "${API}/api/query?type=logs&service=smoke-test" | jq -e ".rows | length >= 1" >/dev/null \
  || fail "log query empty"
pass "log query ok"

say "GET ${API}/api/query?type=metrics&service=smoke-test&name=requests&window=1m"
curl -sSf "${API}/api/query?type=metrics&service=smoke-test&name=requests&window=1m" \
  | jq -e ".series | length >= 1" >/dev/null || fail "metric query empty"
pass "metric query ok"

say "GET ${API}/api/query?type=traces&service=smoke-test"
curl -sSf "${API}/api/query?type=traces&service=smoke-test" \
  | jq -e ".rows | length >= 1" >/dev/null || fail "trace query empty"
pass "trace query ok"

say "GET ${API}/api/datasources"
curl -sSf "${API}/api/datasources" | jq -e ".count >= 1" >/dev/null || fail "datasources empty"
pass "datasources ok"

say "GET ${API}/api/dashboards/overview/panels"
curl -sSf "${API}/api/dashboards/overview/panels" | jq -e ".panels | length >= 1" >/dev/null \
  || fail "overview panels empty"
pass "dashboard panels ok"

say "GET ${API}/api/seed?service=smoke-seed&n=3"
curl -sSf "${API}/api/seed?service=smoke-seed&n=3" | jq -e ".seeded == 3" >/dev/null \
  || fail "seed returned wrong count"
pass "seed endpoint ok"

printf "\n\033[1;32mAll smoke checks passed.\033[0m\n"
