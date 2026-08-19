#!/usr/bin/env bash
set -euo pipefail
DIR="$(cd "$(dirname "$0")" && pwd)"
found=0
for d in $(find "$DIR" -name '*.json' -type f); do
  found=$((found+1))
  echo "==> $d"
  python3 -c "
import json, sys
d = json.load(open('$d'))
for k in ('title', 'panels'):
    if k not in d:
        print('missing key:', k); sys.exit(1)
if not isinstance(d['panels'], list) or len(d['panels']) == 0:
    print('panels must be a non-empty list'); sys.exit(1)
for p in d['panels']:
    for k in ('id', 'title', 'type'):
        if k not in p:
            print('panel missing', k); sys.exit(1)
"
done
for y in $(find "$DIR/provisioning" -name '*.yaml' -type f); do
  echo "==> $y"
  python3 -c "
import yaml, sys
d = yaml.safe_load(open('$y'))
if 'apiVersion' not in d:
    print('missing apiVersion'); sys.exit(1)
"
done
echo "OK ($found dashboards)"
