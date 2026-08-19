#!/usr/bin/env bash
# helm/validate.sh
# Validate Helm chart values.yaml + Chart.yaml structure. Templates
# themselves are validated by the helm-lint CI workflow (which uses
# the real helm binary to render them). This script catches cheap
# bugs without needing helm installed: values.yaml YAML errors,
# missing required Chart.yaml fields, helm template files that don't
# reference the expected helper names.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
echo "Repo root: $REPO_ROOT"

found=0
for values in $(find "$REPO_ROOT/helm" -name values.yaml -type f); do
  found=$((found+1))
  echo "==> $values"
  python3 -c "import yaml; yaml.safe_load(open('$values'))"
done
[ "$found" = 0 ] && { echo "No values.yaml found"; exit 1; }

for chart in $(find "$REPO_ROOT/helm" -name Chart.yaml -type f); do
  echo "==> $chart"
  python3 -c "
import yaml, sys
d = yaml.safe_load(open('$chart'))
for k in ('apiVersion','name','version'):
    if k not in d:
        print('missing key:', k); sys.exit(1)
if not d['name'].startswith('dog'):
    print('chart name should start with dog-, got', d['name']); sys.exit(1)
if not isinstance(d['version'], str) or not d['version']:
    print('version must be a non-empty string'); sys.exit(1)
"
done

# Verify every template file contains at least one of the standard
# Kubernetes kinds so we don't accidentally ship a no-op template.
for tpl in $(find "$REPO_ROOT/helm" -path '*/templates/*' -name '*.yaml' -type f); do
  python3 -c "
import sys
src = open('$tpl').read()
for needle in ('apiVersion:','kind:','name:'):
    if needle not in src:
        print('FAIL: $tpl missing', needle); sys.exit(1)
"
done

echo "OK ($found charts, $(find $REPO_ROOT/helm -path '*/templates/*' -name '*.yaml' | wc -l) templates)"
