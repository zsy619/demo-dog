# ============================================================================
# demo-dog (root) -- unified build / test / run orchestrator
# ============================================================================
#
# This Makefile delegates to backend/Makefile and frontend/package.json so the
# top-level target names are stable even if the underlying tooling changes.
#
# Quick start:
#   make build       # build backend (Go) + frontend (npm) into dist/
#   make run         # start backend on :18080 + frontend on :5173
#   make test        # run backend (go test -race) + frontend (typecheck)
#   make smoke       # run the end-to-end smoke suite (backend must be up)
#   make docker      # build the multi-stage image (default target: backend)
#   make clean       # remove all build artifacts
#
# ============================================================================

# ---- Configuration ----------------------------------------------------------
SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c

PROJECT      := demo-dog
VERSION      ?= 0.1.0

BACKEND_DIR  := backend
FRONTEND_DIR := frontend
DIST_DIR     := dist
RUN_DIR      := .run

BACKEND_PORT  ?= 18080
FRONTEND_PORT ?= 5173

GO     ?= $(shell command -v go 2>/dev/null || echo /usr/local/go/bin/go)
NODE   ?= $(shell command -v node 2>/dev/null || echo /opt/homebrew/bin/node)
NPM    ?= $(shell command -v npm 2>/dev/null || echo /opt/homebrew/bin/npm)
DOCKER ?= $(shell command -v docker 2>/dev/null || echo docker)

GOFLAGS  ?= -trimpath
LDFLAGS  ?= -s -w -X main.version=$(PROJECT)-$(VERSION)

BACKEND_BIN  := $(BACKEND_DIR)/bin/dog-collector
FRONTEND_OUT := $(DIST_DIR)/frontend

DOCKER_TAG        ?= $(PROJECT):$(VERSION)
DOCKERFILE        ?= Dockerfile
DOCKER_BUILD_ARGS ?=

# brew-installed npm uses `#!/usr/bin/env node` and fails unless its sibling
# node binary is on PATH for the subshell. Recipes that call npm prefix the
# invocation with this string so they work even when the caller environment
# is stripped (e.g. CI containers, systemd, IDE task runners).
NPM_RUN = PATH=/opt/homebrew/bin:/usr/local/go/bin:/usr/bin:/bin:$$PATH $(NPM)

# ---- Targets ----------------------------------------------------------------
.DEFAULT_GOAL := help

all: build

# ---- Build ------------------------------------------------------------------
.PHONY: build
build: backend-build frontend-build
	@echo "==> build complete"
	@echo "    backend  : $(BACKEND_BIN)"
	@echo "    frontend : $(FRONTEND_OUT)/"

.PHONY: backend-build
backend-build: ## Build the Go collector binary
	@echo "==> building backend (go)"
	cd $(BACKEND_DIR) && $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o bin/dog-collector ./cmd/dog-collector

.PHONY: frontend-build
frontend-build: ## Build the React + Vite frontend into $(FRONTEND_OUT)
	@echo "==> building frontend (npm run build)"
	@mkdir -p $(FRONTEND_OUT)
	[ -d $(FRONTEND_DIR)/node_modules ] || $(NPM_RUN) install --no-audit --no-fund
	cd $(FRONTEND_DIR) && $(NPM_RUN) run build
	@rm -rf $(FRONTEND_OUT)
	@mkdir -p $(DIST_DIR)
	@cp -R $(FRONTEND_DIR)/dist $(FRONTEND_OUT)
	@echo "    mirrored to $(FRONTEND_OUT)/"

# ---- Test -------------------------------------------------------------------
.PHONY: test
test: backend-test frontend-test
	@echo "==> all tests passed"

.PHONY: backend-test
backend-test: ## Run Go unit tests with the race detector
	cd $(BACKEND_DIR) && $(GO) test -race -count=1 ./...

.PHONY: frontend-test
frontend-test: ## Run TypeScript type-check on the frontend
	[ -d $(FRONTEND_DIR)/node_modules ] || $(NPM_RUN) install --no-audit --no-fund
	cd $(FRONTEND_DIR) && $(NPM_RUN) run typecheck

# ---- Vet / format -----------------------------------------------------------
.PHONY: vet
vet: backend-vet ## Run go vet + frontend typecheck

.PHONY: backend-vet
backend-vet:
	cd $(BACKEND_DIR) && $(GO) vet ./...

.PHONY: fmt
fmt: backend-fmt frontend-fmt ## Format Go + frontend

.PHONY: backend-fmt
backend-fmt:
	cd $(BACKEND_DIR) && gofmt -w .

.PHONY: frontend-fmt
frontend-fmt:
	cd $(FRONTEND_DIR) && $(NPM_RUN) run --if-present format

# ---- Run --------------------------------------------------------------------
.PHONY: run
run: build ## Run backend + frontend in the foreground (Ctrl+C stops both)
	@echo "==> starting backend on :$(BACKEND_PORT)"
	@echo "==> starting frontend on :$(FRONTEND_PORT)"
	@mkdir -p $(RUN_DIR)
	cd $(BACKEND_DIR) && ./bin/dog-collector \
		-addr ":$(BACKEND_PORT)" -workers 8 \
		-seed "checkout,search,inventory,auth,recommend,ads" &
	BACKEND_PID=$$!; \
	cd $(FRONTEND_DIR) && $(NPM_RUN) run dev -- --port $(FRONTEND_PORT) --host 127.0.0.1 &
	FRONTEND_PID=$$!; \
	trap "kill $$BACKEND_PID $$FRONTEND_PID 2>/dev/null || true" INT TERM; \
	wait

.PHONY: run-backend
run-backend: backend-build ## Run only the backend in the foreground
	cd $(BACKEND_DIR) && ./bin/dog-collector -addr ":$(BACKEND_PORT)" -workers 8

.PHONY: run-frontend
run-frontend: ## Run only the frontend dev server in the foreground
	[ -d $(FRONTEND_DIR)/node_modules ] || $(NPM_RUN) install --no-audit --no-fund
	cd $(FRONTEND_DIR) && $(NPM_RUN) run dev -- --port $(FRONTEND_PORT) --host 127.0.0.1

# ---- Smoke ------------------------------------------------------------------
.PHONY: smoke
smoke: ## Run the end-to-end smoke suite (backend must be running)
	API=http://localhost:$(BACKEND_PORT) bash scripts/smoke.sh

.PHONY: smoke-up
smoke-up: build ## Start backend in background, run smoke, stop it
	cd $(BACKEND_DIR) && ./bin/dog-collector \
		-addr ":$(BACKEND_PORT)" -workers 8 \
		-seed "checkout,search,inventory,auth,recommend,ads" &
	BACKEND_PID=$$!; \
	sleep 2; \
	API=http://localhost:$(BACKEND_PORT) bash scripts/smoke.sh; \
	SMOKE_RC=$$?; \
	kill $$BACKEND_PID 2>/dev/null || true; \
	wait $$BACKEND_PID 2>/dev/null || true; \
	exit $$SMOKE_RC

# ---- Docker -----------------------------------------------------------------
.PHONY: docker
docker: ## Build the default Docker image (backend target)
	$(DOCKER) build $(DOCKER_BUILD_ARGS) -f $(DOCKERFILE) -t $(DOCKER_TAG) --target backend .

.PHONY: docker-backend
docker-backend: ## Build the backend-only image
	$(DOCKER) build $(DOCKER_BUILD_ARGS) -f $(DOCKERFILE) -t $(PROJECT)-backend:$(VERSION) --target backend .

.PHONY: docker-frontend
docker-frontend: ## Build the frontend-only image (nginx serving static bundle)
	$(DOCKER) build $(DOCKER_BUILD_ARGS) -f $(DOCKERFILE) -t $(PROJECT)-frontend:$(VERSION) --target frontend .

.PHONY: docker-all
docker-all: ## Build both backend and frontend images
	$(DOCKER) build $(DOCKER_BUILD_ARGS) -f $(DOCKERFILE) -t $(PROJECT)-backend:$(VERSION) --target backend .
	$(DOCKER) build $(DOCKER_BUILD_ARGS) -f $(DOCKERFILE) -t $(PROJECT)-frontend:$(VERSION) --target frontend .

.PHONY: docker-run
docker-run: docker-backend ## Run the backend image locally
	$(DOCKER) run --rm -p 8080:8080 $(PROJECT)-backend:$(VERSION)

.PHONY: compose-up
compose-up: ## Bring up backend + frontend via docker compose (nginx image)
	$(DOCKER) compose up --build

.PHONY: compose-down
compose-down: ## Tear down docker-compose services
	$(DOCKER) compose down

.PHONY: compose-up-caddy
compose-up-caddy: ## Bring up backend + frontend via docker compose (Caddy image)
	$(DOCKER) compose -f docker-compose.caddy.yml up --build

.PHONY: compose-down-caddy
compose-down-caddy: ## Tear down the Caddy-flavoured compose stack
	$(DOCKER) compose -f docker-compose.caddy.yml down

.PHONY: validate-caddyfile
validate-caddyfile: ## Validate deploy/Caddyfile and deploy/Caddyfile.fullstack syntax
	@echo "Validating Caddyfiles with caddy adapt (if caddy is on PATH)..."
	@if command -v caddy >/dev/null 2>&1; then \
		echo "  Caddyfile          :"; caddy adapt --config deploy/Caddyfile         --pretty 2>&1 | head -3 || true; \
		echo "  Caddyfile.fullstack:"; caddy adapt --config deploy/Caddyfile.fullstack --pretty 2>&1 | head -3 || true; \
		echo "  Caddyfile.https    :"; caddy adapt --config deploy/Caddyfile.https     --pretty 2>&1 | head -3 || true; \
	else \
		echo "  caddy binary not on PATH; skipping live validation."; \
		echo "  (install via: brew install caddy) Run manually later with: caddy adapt --config deploy/Caddyfile"; \
	fi

# ---- Install ----------------------------------------------------------------
.PHONY: install
install: ## Install backend (go mod download) + frontend (npm install) deps
	cd $(BACKEND_DIR) && $(GO) mod download || true
	[ -d $(FRONTEND_DIR)/node_modules ] || $(NPM_RUN) install --no-audit --no-fund
	@echo "==> dependencies installed"

.PHONY: deps-backend
deps-backend:
	cd $(BACKEND_DIR) && $(GO) mod download

.PHONY: deps-frontend
deps-frontend:
	[ -d $(FRONTEND_DIR)/node_modules ] || $(NPM_RUN) install --no-audit --no-fund

# ---- Clean ------------------------------------------------------------------
.PHONY: clean
clean: backend-clean frontend-clean ## Remove all build artifacts and run state

.PHONY: backend-clean
backend-clean:
	rm -rf $(BACKEND_DIR)/bin

.PHONY: frontend-clean
frontend-clean:
	rm -rf $(FRONTEND_DIR)/dist $(DIST_DIR)

.PHONY: clean-all
clean-all: clean ## Remove all artifacts + node_modules + .run
	rm -rf $(FRONTEND_DIR)/node_modules $(RUN_DIR)

# ---- Tidy -------------------------------------------------------------------
.PHONY: tidy
tidy: ## Sync Go module files
	cd $(BACKEND_DIR) && $(GO) mod tidy

# ---- Info / help ------------------------------------------------------------
.PHONY: version
version:
	@echo "$(PROJECT) $(VERSION)"

.PHONY: info
info: ## Show tool versions and resolved paths
	@echo "Project       : $(PROJECT) v$(VERSION)"
	@echo "Go            : $$($(GO) version 2>&1 | head -1) (at $(GO))"
	@echo "Node          : $$($(NODE) --version 2>&1 | head -1) (at $(NODE))"
	@echo "npm           : $$(PATH=/opt/homebrew/bin:$$PATH $(NPM) --version 2>&1 | head -1) (at $(NPM))"
	@echo "Docker        : $$($(DOCKER) --version 2>&1 | head -1 || echo "not installed")"
	@echo "Backend port  : $(BACKEND_PORT)"
	@echo "Frontend port : $(FRONTEND_PORT)"

.PHONY: help
help: ## Show this help message
	@echo "$(PROJECT) $(VERSION) - top-level make targets"
	@echo ""
	@printf "  \033[1m%-22s\033[0m %s\n" "build"         "Build backend (Go) + frontend (npm)"
	@printf "  \033[1m%-22s\033[0m %s\n" "build-backend" "Build only the Go collector binary"
	@printf "  \033[1m%-22s\033[0m %s\n" "build-frontend" "Build only the React frontend (outputs dist/frontend)"
	@echo ""
	@printf "  \033[1m%-22s\033[0m %s\n" "test"          "Run all tests (go test -race + tsc)"
	@printf "  \033[1m%-22s\033[0m %s\n" "test-backend"  "Run Go unit tests with -race"
	@printf "  \033[1m%-22s\033[0m %s\n" "test-frontend" "Run TypeScript type-check"
	@printf "  \033[1m%-22s\033[0m %s\n" "vet"           "go vet on backend"
	@printf "  \033[1m%-22s\033[0m %s\n" "fmt"           "gofmt -w on backend"
	@echo ""
	@printf "  \033[1m%-22s\033[0m %s\n" "run"           "Run backend + frontend in foreground"
	@printf "  \033[1m%-22s\033[0m %s\n" "run-backend"   "Run only the backend"
	@printf "  \033[1m%-22s\033[0m %s\n" "run-frontend"  "Run only the frontend"
	@echo ""
	@printf "  \033[1m%-22s\033[0m %s\n" "smoke"         "Run end-to-end smoke (backend must be up)"
	@printf "  \033[1m%-22s\033[0m %s\n" "smoke-up"      "Start backend, run smoke, stop it"
	@echo ""
	@printf "  \033[1m%-22s\033[0m %s\n" "docker"        "Build the default Docker image (backend)"
	@printf "  \033[1m%-22s\033[0m %s\n" "docker-backend" "Build backend-only image"
	@printf "  \033[1m%-22s\033[0m %s\n" "docker-frontend" "Build frontend-only image"
	@printf "  \033[1m%-22s\033[0m %s\n" "docker-all"    "Build both images"
	@printf "  \033[1m%-22s\033[0m %s\n" "docker-run"    "Run backend image on :8080"
	@printf "  \033[1m%-22s\033[0m %s\n" "compose-up"    "Bring up via docker compose (nginx)"
	@printf "  \033[1m%-22s\033[0m %s\n" "compose-down"  "Tear down docker compose"
	@printf "  \033[1m%-22s\033[0m %s\n" "compose-up-caddy" "Bring up via docker compose (Caddy)"
	@printf "  \033[1m%-22s\033[0m %s\n" "validate-caddyfile" "Validate deploy/Caddyfile*"
	@echo ""
	@printf "  \033[1m%-22s\033[0m %s\n" "install"       "Install backend + frontend deps"
	@printf "  \033[1m%-22s\033[0m %s\n" "tidy"          "go mod tidy"
	@printf "  \033[1m%-22s\033[0m %s\n" "clean"         "Remove build artifacts"
	@printf "  \033[1m%-22s\033[0m %s\n" "clean-all"     "Remove build artifacts + node_modules + .run"
	@printf "  \033[1m%-22s\033[0m %s\n" "info"          "Show tool versions + resolved paths"
	@printf "  \033[1m%-22s\033[0m %s\n" "version"       "Print $(PROJECT) version"
	@printf "  \033[1m%-22s\033[0m %s\n" "help"          "Show this help"
	@echo ""
	@echo "Override variables: BACKEND_PORT FRONTEND_PORT VERSION DOCKER_TAG DOCKER_BUILD_ARGS"
