#!/usr/bin/env bash
set -euo pipefail
F="$(cd "$(dirname "$0")/../../.." && pwd)/docs/openapi.json"
if [ ! -f "$F" ]; then
  echo "missing $F; run: go run ./cmd/gen-openapi" >&2
  exit 1
fi
python3 -c "
import json, sys
spec = json.load(open('$F'))
assert spec['openapi'].startswith('3.'), 'openapi version'
assert spec['info']['title'], 'title'
assert spec['info']['version'], 'version'
assert 'paths' in spec and len(spec['paths']) > 0, 'paths'
required = ['/api/health', '/api/v1/query', '/v1/logs', '/api/v1/rules']
for p in required:
    assert p in spec['paths'], 'missing path: ' + p
print('OK paths=' + str(len(spec['paths'])))
"
