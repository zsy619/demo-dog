#!/usr/bin/env bash
# scripts/dev.sh
#
# One-shot helper for the DOG demo. It handles pre-flight checks, dependency
# installation, parallel backend + frontend startup, status checks, and
# graceful shutdown. Designed to be invoked from the project root.
#
# Usage:
#   ./scripts/dev.sh start         # install + run backend and frontend
#   ./scripts/dev.sh start --no-install
#   ./scripts/dev.sh backend       # only the backend
#   ./scripts/dev.sh frontend      # only the frontend
#   ./scripts/dev.sh status        # ping backend + frontend
#   ./scripts/dev.sh smoke         # run scripts/smoke.sh
#   ./scripts/dev.sh stop          # kill any running watchers
#   ./scripts/dev.sh logs [name]   # tail a background log
#   ./scripts/dev.sh clean         # remove build artifacts + node_modules
#   ./scripts/dev.sh help

set -euo pipefail

# ----------------------------------------------------------------------------
# Config (overridable via environment)
# ----------------------------------------------------------------------------
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="${ROOT_DIR}/backend"
FRONTEND_DIR="${ROOT_DIR}/frontend"
LOG_DIR="${ROOT_DIR}/.run"

# If a previous `start` left a .run/env file, prefer its values so subsequent
# read-only commands (status/smoke/stop) auto-detect the right ports. This
# must run BEFORE the defaults below, otherwise defaults overwrite them.
if [[ -f "${LOG_DIR}/env" ]]; then
  # shellcheck source=/dev/null
  source "${LOG_DIR}/env" 2>/dev/null || true
fi

BACKEND_PORT="${BACKEND_PORT:-18080}"
FRONTEND_PORT="${FRONTEND_PORT:-5173}"

SEED_SERVICES="${SEED_SERVICES:-checkout,search,inventory,auth,recommend,ads}"

WORKERS="${WORKERS:-8}"
BINARY_NAME="dog-collector"

mkdir -p "${LOG_DIR}"

# Persist chosen ports so subsequent commands (status/smoke/stop) auto-detect.
# We only write this when start actually launches services (see start_all),
# so read-only commands that re-invoke dev.sh do not clobber previously
# saved ports with the default values.

# ----------------------------------------------------------------------------
# Styling
# ----------------------------------------------------------------------------
if [[ -t 1 ]]; then
  C_RESET="\033[0m"
  C_BOLD="\033[1m"
  C_DIM="\033[2m"
  C_RED="\033[1;31m"
  C_GREEN="\033[1;32m"
  C_YELLOW="\033[1;33m"
  C_BLUE="\033[1;34m"
  C_CYAN="\033[1;36m"
  C_MAGENTA="\033[1;35m"
else
  C_RESET=""; C_BOLD=""; C_DIM=""; C_RED=""; C_GREEN=""; C_YELLOW=""
  C_BLUE=""; C_CYAN=""; C_MAGENTA=""
fi

ok()      { printf "${C_GREEN}OK${C_RESET} %s\n" "$*"; }
info()    { printf "${C_CYAN}info${C_RESET}  %s\n" "$*"; }
warn()    { printf "${C_YELLOW}!${C_RESET}     %s\n" "$*"; }
err()     { printf "${C_RED}FAIL${C_RESET} %s\n" "$*" >&2; }
title()   { printf "\n${C_BOLD}${C_MAGENTA}== %s ==${C_RESET}\n" "$*"; }
section() { printf "\n${C_BOLD}${C_BLUE}-- %s --${C_RESET}\n" "$*"; }

# ----------------------------------------------------------------------------
# Path discovery (Go / Node / npm; tolerates non-PATH defaults)
# ----------------------------------------------------------------------------
find_executable() {
  local name="$1"; shift
  local candidates=("$@")
  if command -v "$name" >/dev/null 2>&1; then
    command -v "$name"
    return 0
  fi
  for c in "${candidates[@]}"; do
    if [[ -x "$c" ]]; then echo "$c"; return 0; fi
  done
  return 1
}

GO_BIN=""
NODE_BIN=""
NPM_BIN=""

resolve_tools() {
  GO_BIN=$(find_executable go /usr/local/go/bin/go /opt/homebrew/bin/go) || GO_BIN=""; export GO_BIN
  NODE_BIN=$(find_executable node /opt/homebrew/bin/node /usr/local/bin/node) || NODE_BIN=""; export NODE_BIN
  NPM_BIN=$(find_executable npm /opt/homebrew/bin/npm /usr/local/bin/npm) || NPM_BIN=""; export NPM_BIN

  if [[ -n "${GO_BIN}" ]]; then export PATH="$(dirname "${GO_BIN}"):${PATH}"; fi
  if [[ -n "${NODE_BIN}" ]]; then export PATH="$(dirname "${NODE_BIN}"):${PATH}"; fi
  if [[ -n "${NPM_BIN}" ]]; then export PATH="$(dirname "${NPM_BIN}"):${PATH}"; fi
}

preflight() {
  resolve_tools
  section "preflight checks"

  for pair in \
    "go:Go" \
    "node:Node.js" \
    "npm:npm" \
  ; do
    cmd="${pair%%:*}"; label="${pair##*:}"
    if command -v "$cmd" >/dev/null 2>&1; then
      case "$cmd" in
        go) ver="$("$cmd" version 2>/dev/null || echo unknown)" ;;
        *)  ver="$("$cmd" --version 2>/dev/null || true | head -1)" ;;
      esac
      ok "$label: $ver"
    else
      warn "$label: not found in PATH"
    fi
  done

  if [[ -d "${BACKEND_DIR}" && -f "${BACKEND_DIR}/go.mod" ]]; then
    ok "backend directory: ${BACKEND_DIR}"
  else
    err "backend directory missing: ${BACKEND_DIR}"
    return 1
  fi

  if [[ -d "${FRONTEND_DIR}" && -f "${FRONTEND_DIR}/package.json" ]]; then
    ok "frontend directory: ${FRONTEND_DIR}"
  else
    err "frontend directory missing: ${FRONTEND_DIR}"
    return 1
  fi

  if [[ -z "${GO_BIN}" ]]; then err "Go is required to run the backend"; return 1; fi
  if [[ -z "${NODE_BIN}" || -z "${NPM_BIN}" ]]; then err "Node.js + npm are required to run the frontend"; return 1; fi
}

# ----------------------------------------------------------------------------
# Dependency installation
# ----------------------------------------------------------------------------
install_deps() {
  section "installing dependencies"

  if [[ ! -d "${BACKEND_DIR}/bin" ]] || [[ ! -x "${BACKEND_DIR}/bin/${BINARY_NAME}" ]]; then
    info "building backend (go build)"
    ( cd "${BACKEND_DIR}" && "${GO_BIN}" build -trimpath -ldflags="-s -w" -o "bin/${BINARY_NAME}" ./cmd/dog-collector )
    ok "backend binary: ${BACKEND_DIR}/bin/${BINARY_NAME}"
  else
    info "backend binary already built (use ./scripts/dev.sh clean to rebuild)"
  fi

  if [[ ! -d "${FRONTEND_DIR}/node_modules" ]]; then
    info "installing frontend deps (npm install)"
    ( cd "${FRONTEND_DIR}" && "${NPM_BIN}" install --no-audit --no-fund )
    ok "frontend deps installed"
  else
    info "frontend deps already installed (use ./scripts/dev.sh clean to reinstall)"
  fi
}

# ----------------------------------------------------------------------------
# Lifecycle: start / stop / status
# ----------------------------------------------------------------------------
start_backend() {
  local mode="${1:-detach}"
  local logfile="${LOG_DIR}/backend.log"
  local pidfile="${LOG_DIR}/backend.pid"

  if [[ -f "${pidfile}" ]] && kill -0 "$(cat "${pidfile}")" 2>/dev/null; then
    warn "backend already running (pid $(cat "${pidfile}"))"
    return 0
  fi

  if [[ ! -x "${BACKEND_DIR}/bin/${BINARY_NAME}" ]]; then
    info "backend binary not present, building..."
    ( cd "${BACKEND_DIR}" && "${GO_BIN}" build -trimpath -o "bin/${BINARY_NAME}" ./cmd/dog-collector )
  fi

  info "starting backend on :${BACKEND_PORT} (workers=${WORKERS}, seed=${SEED_SERVICES})"
  if [[ "${mode}" == "detach" ]]; then
    nohup "${BACKEND_DIR}/bin/${BINARY_NAME}" \
      -addr ":${BACKEND_PORT}" \
      -workers "${WORKERS}" \
      -seed "${SEED_SERVICES}" \
      > "${logfile}" 2>&1 &
    echo $! > "${pidfile}"
    sleep 1
    ok "backend started (pid $(cat "${pidfile}"), log ${logfile})"
  else
    exec "${BACKEND_DIR}/bin/${BINARY_NAME}" \
      -addr ":${BACKEND_PORT}" \
      -workers "${WORKERS}" \
      -seed "${SEED_SERVICES}"
  fi
}

start_frontend() {
  local mode="${1:-detach}"
  local logfile="${LOG_DIR}/frontend.log"
  local pidfile="${LOG_DIR}/frontend.pid"

  if [[ -f "${pidfile}" ]] && kill -0 "$(cat "${pidfile}")" 2>/dev/null; then
    warn "frontend already running (pid $(cat "${pidfile}"))"
    return 0
  fi

  if [[ ! -d "${FRONTEND_DIR}/node_modules" ]]; then
    info "node_modules not present, installing..."
    ( cd "${FRONTEND_DIR}" && "${NPM_BIN}" install --no-audit --no-fund )
  fi

  info "starting frontend on :${FRONTEND_PORT}"
  if [[ "${mode}" == "detach" ]]; then
    ( cd "${FRONTEND_DIR}" && nohup "${NPM_BIN}" run dev -- --port "${FRONTEND_PORT}" --host 127.0.0.1 > "${logfile}" 2>&1 & echo $! > "${pidfile}" )
    sleep 1
    ok "frontend started (pid $(cat "${pidfile}"), log ${logfile})"
  else
    ( cd "${FRONTEND_DIR}" && exec "${NPM_BIN}" run dev -- --port "${FRONTEND_PORT}" --host 127.0.0.1 )
  fi
}

start_all() {
  local no_install="${1:-}"
  preflight
  if [[ -z "${no_install}" ]]; then
    install_deps
  fi
  section "starting services"
  start_backend
  start_frontend

  # Persist the actually-used ports so subsequent commands (status/smoke/stop)
  # can auto-detect without the caller needing to set env vars again.
  cat > "${LOG_DIR}/env" <<ENVFILE
BACKEND_PORT=${BACKEND_PORT}
FRONTEND_PORT=${FRONTEND_PORT}
ENVFILE

  section "ready"
  info "backend:  ${C_BOLD}http://localhost:${BACKEND_PORT}/api/health${C_RESET}"
  info "frontend: ${C_BOLD}http://localhost:${FRONTEND_PORT}/${C_RESET}"
  info "logs:     ${LOG_DIR}/"
  info ""
  info "tip: run ${C_BOLD}./scripts/dev.sh smoke${C_RESET} to verify everything works"
  info "tip: run ${C_BOLD}./scripts/dev.sh stop${C_RESET} to shut down both"
}

stop_one() {
  local name="$1"; local pidfile="${LOG_DIR}/${name}.pid"
  if [[ ! -f "${pidfile}" ]]; then
    info "no pidfile for ${name} (not running)"
    return 0
  fi
  local pid; pid="$(cat "${pidfile}")"
  if kill -0 "${pid}" 2>/dev/null; then
    info "stopping ${name} (pid ${pid})"
    pkill -P "${pid}" 2>/dev/null || true
    kill "${pid}" 2>/dev/null || true
    sleep 0.5
    kill -9 "${pid}" 2>/dev/null || true
    ok "${name} stopped"
  else
    info "${name} pid ${pid} was not running"
  fi
  rm -f "${pidfile}"
}

stop_all() {
  section "stopping services"
  stop_one frontend
  stop_one backend
  ok "all stopped"
}

status() {
  section "status"

  local backend_pid="" frontend_pid="" backend_state="stopped" frontend_state="stopped"
  if [[ -f "${LOG_DIR}/backend.pid" ]]; then
    backend_pid=$(cat "${LOG_DIR}/backend.pid" 2>/dev/null || true)
    if [[ -n "${backend_pid}" ]] && kill -0 "${backend_pid}" 2>/dev/null; then
      backend_state="running (pid ${backend_pid})"
    fi
  fi
  if [[ -f "${LOG_DIR}/frontend.pid" ]]; then
    frontend_pid=$(cat "${LOG_DIR}/frontend.pid" 2>/dev/null || true)
    if [[ -n "${frontend_pid}" ]] && kill -0 "${frontend_pid}" 2>/dev/null; then
      frontend_state="running (pid ${frontend_pid})"
    fi
  fi

  printf "%-9s %s\n" "backend"  "${backend_state}"
  printf "%-9s %s\n" "frontend" "${frontend_state}"

  section "health probes"
  if [[ "${backend_state}" != stopped* ]] && [[ -n "${backend_pid}" ]]; then
    if wait_for_http "http://localhost:${BACKEND_PORT}/api/health" 5; then
      local eng
      eng=$(curl -sS "http://localhost:${BACKEND_PORT}/api/health" | python3 -c "import json,sys;d=json.load(sys.stdin);print('status=',d.get('status'),' services=',d.get('engine',{}).get('services'))" 2>/dev/null || echo "ok")
      ok "backend health: ${eng}"
    else
      warn "backend pid alive but /api/health unreachable after 5s"
    fi
  else
    info "backend not running, skipping health probe"
  fi

  if [[ "${frontend_state}" != stopped* ]] && [[ -n "${frontend_pid}" ]]; then
    if wait_for_http "http://localhost:${FRONTEND_PORT}/" 5; then
      ok "frontend reachable on :${FRONTEND_PORT}"
    else
      warn "frontend pid alive but :${FRONTEND_PORT} unreachable after 5s"
    fi
  else
    info "frontend not running, skipping health probe"
  fi
}

wait_for_http() {
  local url="$1"; local max="${2:-5}"; local i
  for ((i=0; i<max; i++)); do
    if curl -sS --max-time 1 -o /dev/null -w "%{http_code}" "$url" 2>/dev/null | grep -qE "^(200|304)$"; then
      return 0
    fi
    sleep 1
  done
  return 1
}

smoke() {
  section "running end-to-end smoke"
  if [[ ! -f "${ROOT_DIR}/scripts/smoke.sh" ]]; then
    err "smoke.sh not found at ${ROOT_DIR}/scripts/smoke.sh"
    return 1
  fi
  API="http://localhost:${BACKEND_PORT}" bash "${ROOT_DIR}/scripts/smoke.sh"
}

tail_log() {
  local name="${1:-backend}"
  local logfile="${LOG_DIR}/${name}.log"
  if [[ ! -f "${logfile}" ]]; then
    err "no log file: ${logfile}"
    return 1
  fi
  info "tailing ${logfile} (Ctrl+C to stop)"
  tail -f "${logfile}"
}

clean() {
  section "cleaning build artifacts"
  stop_all || true

  if [[ -d "${BACKEND_DIR}/bin" ]]; then
    rm -rf "${BACKEND_DIR}/bin"
    ok "removed backend/bin"
  fi
  if [[ -d "${FRONTEND_DIR}/node_modules" ]]; then
    rm -rf "${FRONTEND_DIR}/node_modules"
    ok "removed frontend/node_modules"
  fi
  if [[ -d "${FRONTEND_DIR}/dist" ]]; then
    rm -rf "${FRONTEND_DIR}/dist"
    ok "removed frontend/dist"
  fi
  if [[ -d "${LOG_DIR}" ]]; then
    rm -rf "${LOG_DIR}"
    ok "removed .run/"
  fi
  ok "clean done"
}

open_browser() {
  local url="${1:-http://localhost:${FRONTEND_PORT}/}"
  if command -v open >/dev/null 2>&1; then
    open "${url}"
  elif command -v xdg-open >/dev/null 2>&1; then
    xdg-open "${url}"
  else
    info "open this URL in your browser: ${url}"
  fi
}

help() {
  cat <<EOF
${C_BOLD}DOG -- one-shot dev helper${C_RESET}

Usage:
  ./scripts/dev.sh <command> [args]

Commands:
  ${C_BOLD}start [--no-install] [--open]${C_RESET}    install (optional) + start backend + frontend
  ${C_BOLD}backend [-f]${C_RESET}                       start only the backend (foreground if -f)
  ${C_BOLD}frontend [-f]${C_RESET}                      start only the frontend (foreground if -f)
  ${C_BOLD}status${C_RESET}                             show running status + health probes
  ${C_BOLD}smoke${C_RESET}                              run scripts/smoke.sh
  ${C_BOLD}stop${C_RESET}                               stop both backend and frontend
  ${C_BOLD}logs [backend|frontend]${C_RESET}           tail a log file (default: backend)
  ${C_BOLD}clean${C_RESET}                              remove build artifacts + node_modules + pids
  ${C_BOLD}browser${C_RESET}                            open the frontend in the default browser
  ${C_BOLD}help${C_RESET}                               show this message

Environment variables:
  BACKEND_PORT       default: 18080
  FRONTEND_PORT      default: 5173
  SEED_SERVICES      default: checkout,search,inventory,auth,recommend,ads
  WORKERS            default: 8

Examples:
  ./scripts/dev.sh start --open       # install + start everything, then open the browser
  ./scripts/dev.sh start --no-install # start without reinstalling deps
  ./scripts/dev.sh backend -f         # run backend in the foreground
  ./scripts/dev.sh status             # see what is running
  ./scripts/dev.sh smoke              # run end-to-end checks
  ./scripts/dev.sh logs frontend      # tail the frontend log
  ./scripts/dev.sh stop               # shut down both
EOF
}

cmd="${1:-help}"
shift || true

case "${cmd}" in
  start)
    no_install=""
    do_open=""
    while [[ $# -gt 0 ]]; do
      case "$1" in
        --no-install) no_install="1"; shift ;;
        --open)       do_open="1";    shift ;;
        *) err "unknown flag: $1"; exit 1 ;;
      esac
    done
    start_all "${no_install}"
    [[ -n "${do_open}" ]] && open_browser
    ;;
  backend)
    mode="detach"
    if [[ "${1:-}" == "-f" ]]; then mode="foreground"; fi
    preflight
    install_deps
    start_backend "${mode}"
    ;;
  frontend)
    mode="detach"
    if [[ "${1:-}" == "-f" ]]; then mode="foreground"; fi
    preflight
    install_deps
    start_frontend "${mode}"
    ;;
  status) status ;;
  smoke)  smoke  ;;
  stop|stop-all) stop_all ;;
  logs)   tail_log "${1:-backend}" ;;
  clean)  clean  ;;
  browser) open_browser ;;
  help|-h|--help) help ;;
  *)
    err "unknown command: ${cmd}"
    echo "run ${C_BOLD}./scripts/dev.sh help${C_RESET} for usage" >&2
    exit 1
    ;;
esac
