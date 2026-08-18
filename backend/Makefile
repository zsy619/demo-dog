# Legacy backend Makefile.
#
# This file is preserved for backwards compatibility; the recommended entry
# point is the unified root-level Makefile (`make` from the project root).
# Both files share the same recipe for `build` / `test` / `run`.
#
# Equivalent root-level targets:
#   make build           => builds the entire stack
#   make build-backend   => builds only the Go collector
#   make test-backend    => runs Go tests
#   make run-backend     => runs only the backend
#
# See ../Makefile for the full list of supported targets.

.PHONY: all build run test fmt vet clean tidy

GOFLAGS := -trimpath
LDFLAGS := -s -w -X main.version=demo-dog-0.1.0

all: build

build:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/dog-collector ./cmd/dog-collector

run:
	go run ./cmd/dog-collector -seed "checkout,search,inventory,auth,recommend,ads" -workers 8

dev: build run

test:
	go test -race -count=1 ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/
