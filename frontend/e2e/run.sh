#!/bin/bash
# Round 24 E2E: build backend, run smoke + tenants scripts.
set -euo pipefail
cd "$(dirname "$0")/../../backend"
export PATH=/opt/homebrew/bin:/usr/local/go/bin:/usr/bin:/bin:$PATH
go build -o /tmp/dog ./cmd/dog-collector
cd ../frontend/e2e
./smoke.mjs
./tenants.mjs
echo "E2E PASSED"
