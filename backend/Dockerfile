# Multi-stage Dockerfile for the DOG collector backend.
#
# Stage 1: build a static Go binary using a pinned toolchain.
# Stage 2: run it on a minimal Alpine image so the deployed artifact is small.
#
# Round 52 adds: health-check, OOM score, two exposed ports
# (8080 ingest, 9100 admin/metrics), explicit data volume.

FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache go.mod first for faster incremental builds.
COPY go.mod go.sum* ./
RUN go mod download || true

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/dog-collector ./cmd/dog-collector

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 dog && \
    mkdir -p /var/lib/demo-dog && chown -R dog:dog /var/lib/demo-dog
COPY --from=build /out/dog-collector /usr/local/bin/dog-collector
USER dog
VOLUME ["/var/lib/demo-dog"]
EXPOSE 8080 9100
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:9100/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/dog-collector"]
CMD ["-addr", ":8080", "-admin-addr", ":9100", "-data-dir", "/var/lib/demo-dog", "-workers", "8"]
