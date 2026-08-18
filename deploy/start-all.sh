#!/bin/sh
# =============================================================================
# start-all.sh -- supervisor for the full-stack demo-dog image
#
# Starts dog-collector in the background on :8080, then execs nginx in the
# foreground. Traps SIGTERM/SIGINT to give the backend a chance to drain.
# =============================================================================

set -eu

BACKEND_ADDR="${BACKEND_ADDR:-:8080}"
FRONTEND_PORT="${FRONTEND_PORT:-8080}"
SEED_SERVICES="${SEED_SERVICES:-checkout,search,inventory,auth,recommend,ads}"
WORKERS="${WORKERS:-8}"

# 1) Launch the collector
dog-collector \
    -addr "${BACKEND_ADDR}" \
    -workers "${WORKERS}" \
    -seed "${SEED_SERVICES}" &
BACKEND_PID=$!

# 2) Forward termination signals
shutdown() {
    echo "[start-all] caught signal, stopping backend (pid ${BACKEND_PID})"
    kill "${BACKEND_PID}" 2>/dev/null || true
    wait "${BACKEND_PID}" 2>/dev/null || true
    exit 0
}
trap shutdown TERM INT

# 3) Wait for backend to be ready (max 10s)
i=0
while [ "$i" -lt 10 ]; do
    if wget -qO- "http://127.0.0.1:8080/api/health" >/dev/null 2>&1; then
        echo "[start-all] backend ready"
        break
    fi
    sleep 1
    i=$((i + 1))
done

# 4) Launch nginx in foreground
echo "[start-all] starting nginx on :${FRONTEND_PORT}"
exec nginx -g "daemon off;"
