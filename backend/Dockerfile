# Multi-stage Dockerfile for the DOG collector backend.
#
# Stage 1: build a static Go binary using a pinned toolchain.
# Stage 2: run it on a minimal Alpine image so the deployed artifact is small.

FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache go.mod first for faster incremental builds.
COPY go.mod go.sum* ./
RUN go mod download || true

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/dog-collector ./cmd/dog-collector

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 dog
COPY --from=build /out/dog-collector /usr/local/bin/dog-collector
USER dog
EXPOSE 8080
ENTRYPOINT ["dog-collector"]
CMD ["-addr", ":8080", "-workers", "8"]
