# Releasing

The project follows [Semantic Versioning 2.0.0](https://semver.org/).
Until the project exits demo state the major version stays at 0 and
the minor version signals breaking changes.

## Version cadence

| Bump | When |
|------|------|
| `MAJOR` (x.0.0) | Incompatible API / wire-format change. Currently unused. |
| `MINOR` (0.x.0) | New endpoint, new SDK option, new RBAC role, new query field. Backward compatible with the previous minor. |
| `PATCH` (0.0.x) | Bug fix, perf improvement, internal refactor. No public API change. |

## Tagging

```bash
git tag -a v0.1.0 -m "Round 21: frontend enterprise slice"
git push origin main --tags
```

Pushing a tag triggers `.github/workflows/release.yml` which:

1. Builds a multi-arch (linux/amd64 + linux/arm64) Docker image via
   `docker buildx`.
2. Pushes it to `ghcr.io/<owner>/dog-collector:<tag>` and `:latest`.
3. Creates a GitHub release with auto-generated notes.

## Verifying locally

```bash
docker buildx build --platform linux/amd64,linux/arm64 -t dog-collector:test --load backend
docker run --rm -p 18080:18080 dog-collector:test
```

## Required secrets

| Secret | Purpose |
|--------|---------|
| `GITHUB_TOKEN` | Provided automatically by Actions; used to publish to GHCR and create the release. |

No other secrets are required; the release workflow does not push to
Docker Hub or any other registry by default.

## Versioning the SDK

`sdk/otlp-go` carries its own module path (`github.com/zsy619/demo-dog/sdk/otlp-go`).
When the SDK changes incompatibly with the backend, the SDK bumps its
own minor version. Until then the SDK version moves in lockstep with
the backend.
