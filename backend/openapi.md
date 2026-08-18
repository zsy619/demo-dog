# API contract (openapi.yaml)

The collector exposes a single OpenAPI 3.1 spec that documents every
public endpoint, the request/response shapes, the auth model, and the
RBAC roles required for admin endpoints.

## Why a hand-written spec

Codegen from Go struct tags is appealing but introduces another
build-time dependency. The demo has 28 endpoints with stable shapes;
keeping the spec in version control means:

- A reviewer can see the API surface in a single file.
- CI can lint it without invoking Go.
- The frontend can pin against it without running codegen.

## Lint it

Validate the YAML structure:

```bash
python3 -c "import yaml; yaml.safe_load(open('backend/openapi.yaml'))"
```

Lint against the OpenAPI 3.1 schema (Redocly CLI):

```bash
npx @redocly/cli@latest lint backend/openapi.yaml
```

## Generate TypeScript types

```bash
npx openapi-typescript backend/openapi.yaml -o frontend/src/types/api.generated.ts
```

The frontend still uses the hand-written `frontend/src/types/api.ts`
for ergonomics (e.g. derived union types the spec cannot express);
the generated file is meant for CI drift detection, not import.

## Roles

Three roles apply per API key:

| Role   | Reads | Writes | Admin |
|--------|-------|--------|-------|
| reader | yes   | no     | no    |
| writer | yes   | yes    | no    |
| admin  | yes   | yes    | yes   |

Register via `-api-keys k1:admin:ops,k2:writer:checkout,k3:reader:grafana`
or `DOG_API_KEYS` env var. See `internal/api/auth.go` for the canonical
parser.
